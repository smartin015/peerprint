// Package transport provides the base communication layer
package transport

import (
  "log"
  "time"
  "context"
  "fmt"
  "math/rand"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	rpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/smartin015/peerprint/p2pgit/topic"
	"github.com/smartin015/peerprint/p2pgit/cmd"
	"github.com/smartin015/peerprint/p2pgit/discovery"
	"google.golang.org/protobuf/proto"
)

const (
  ServiceName = "PeerPrintService"
)

type Interface interface {
  Register(protocol string, srv interface{}) error
  Run(context.Context)
  ID() string
  Sign([]byte) ([]byte, error)
  PubKey() crypto.PubKey

  Publish(topic string, msg proto.Message) error
  OnMessage() <-chan topic.TopicMsg
  OnCommand() <-chan proto.Message

  ReplyCmd(proto.Message) error

  GetRandomNeighbor() (peer.ID, error)
  Call(pid peer.ID, method string, req proto.Message, rep proto.Message) error 
}

type Opts struct {
  PubsubAddr string
  CmdRepAddr string
  Rendezvous string
  Local bool
  PrivKey crypto.PrivKey
  PubKey crypto.PubKey
  PSK pnet.PSK
  ConnectTimeout time.Duration
  Topics []string
  Service interface{}
}

type Transport struct {
  // Initial config options
  opts *Opts

  // For RAFT consensus state sync
  leader bool

  // Discoverable PubSub transport layer so
  // consensus can scale
  discovery *discovery.Discovery
  host host.Host
  pubsub *pubsub.PubSub
  recvChan chan topic.TopicMsg
  cmdRecv chan proto.Message
  cmdSend chan<- proto.Message
  errChan chan error
  pubChan map[string] chan<- proto.Message
  protocol protocol.ID
  server *rpc.Server
  l *log.Logger
}

func New(opts *Opts, ctx context.Context, logger *log.Logger) (Interface, error) {
  pid := libp2p.Identity(opts.PrivKey)

  // Initialize base pubsub infra
	h, err := libp2p.New(libp2p.ListenAddrStrings(opts.PubsubAddr), libp2p.PrivateNetwork(opts.PSK), pid)
	if err != nil {
    return nil, fmt.Errorf("PubSub host creation failure: %w", err)
	}
  // TODO switch to using pubsub.WithDiscovery
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
    return nil, fmt.Errorf("GossipSub creation failure: %w", err)
  }

  // Initialize discovery service for pubsub
	disco := discovery.DHT
	if opts.Local {
		disco = discovery.MDNS
	}
	d := discovery.New(ctx, disco, h, opts.Rendezvous, logger)

  // Initialize command service
	cmdRecv := make(chan proto.Message, 5)
	errChan := make(chan error, 5)
	cmdSend := cmd.New(opts.CmdRepAddr, cmdRecv, errChan)

  s := &Transport{
    opts: opts,
    discovery: d,
    pubsub: ps,
    host: h,
    recvChan: make(chan topic.TopicMsg),
    cmdRecv: cmdRecv,
    cmdSend: cmdSend,
    errChan: errChan,
    pubChan: make(map[string] chan<- proto.Message),
    l: logger,
  }

  // Join topics
  opts.Topics = append(opts.Topics)
  for _, t := range(opts.Topics) {
    c, err := topic.NewTopicChannel(ctx, s.recvChan, s.ID(), ps, t, s.errChan)
    if err != nil {
      return nil, fmt.Errorf("failed to join topic %s: %w", t, err)
    }
    logger.Printf("Joined topic: %q\n", t)
    s.pubChan[t] = c
  }

  return s, nil
}

func (s *Transport) Register(pid string, srv interface{}) error {
  // RPC server for point-to-point requests
  s.protocol = protocol.ID(pid)
  s.server = rpc.NewServer(s.host, s.protocol)
  return s.server.RegisterName(ServiceName, srv)
}

func (s *Transport) Sign(b []byte) ([]byte, error) {
  return s.opts.PrivKey.Sign(b)
}

func (s *Transport) PubKey() crypto.PubKey {
  return s.opts.PubKey
}

func (s *Transport) OnMessage() <-chan topic.TopicMsg {
  return s.recvChan
}

func (s *Transport) OnCommand() <-chan proto.Message {
  return s.cmdRecv
}

func (s *Transport) ReplyCmd(msg proto.Message) error {
  select {
  case s.cmdSend<- msg:
  default:
    return fmt.Errorf("ReplyCmd() error: channel full")
  }
  return nil
}

func (s *Transport) Publish(topic string, msg proto.Message) error {
  select {
  case s.pubChan[topic] <- msg:
  default:
    return fmt.Errorf("Publish() error: channel full")
  }
  return nil
}

func (s *Transport) Run(ctx context.Context) {
	s.l.Printf("Discovering pubsub peers (self ID %v, timeout %v)\n", s.ID(), s.opts.ConnectTimeout)
	connectCtx, cancel := context.WithTimeout(ctx, s.opts.ConnectTimeout)
	defer cancel()

  go s.discovery.Run()
	if err := s.discovery.AwaitReady(connectCtx); err != nil {
		panic(fmt.Errorf("Error connecting to peers: %w", err))
	} else {
		s.l.Println("Peers found; initial discovery complete")
	}
}

func (s *Transport) ID() string {
  return s.host.ID().String()
}

func (t *Transport) Call(pid peer.ID, method string, req proto.Message, rep proto.Message) error {
  c := rpc.NewClient(t.host, t.protocol)
  if err := c.Call(pid, ServiceName, "GetState", req, rep); err != nil {
    return fmt.Errorf("Call(%s, %+v, %+v, _) error: %w", pid, method, req, err)
  } 
  return nil
}

func (t *Transport) GetRandomNeighbor() (peer.ID, error) {
  pp := t.host.Peerstore().Peers()
  if len(pp) <= 1 {
    return "", fmt.Errorf("Peerstore is empty")
  }
  i := rand.Intn(len(pp))
  if pp[i].String() == t.ID() {
    return pp[(i+1) % len(pp)], nil
  } else {
    return pp[i], nil
  }
}

