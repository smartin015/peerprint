// Package transport provides the base communication layer
package transport

import (
  "time"
  "context"
  "fmt"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
  ma "github.com/multiformats/go-multiaddr"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	rpc "github.com/libp2p/go-libp2p-gorpc"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/smartin015/peerprint/p2pgit/pkg/topic"
	"github.com/smartin015/peerprint/p2pgit/pkg/discovery"
  "github.com/smartin015/peerprint/p2pgit/pkg/log"
	"google.golang.org/protobuf/proto"
)

const (
  ServiceName = "PeerPrintService"
  MaxMultiAddr = 50
)

type Interface interface {
  Register(protocol string, srv interface{}) error
  Run(context.Context)
  ID() string
  Sign(proto.Message) ([]byte, error)
  Verify(m proto.Message, signer string, data []byte) (bool, error)
  PubKey() crypto.PubKey

  Publish(topic string, msg proto.Message) error
  OnMessage() <-chan topic.TopicMsg

  GetPeers() peer.IDSlice
  GetPeerAddresses() []*peer.AddrInfo
  AddTempPeer(*peer.AddrInfo)
  Call(ctx context.Context, pid peer.ID, method string, req proto.Message, rep proto.Message) error 
  Stream(ctx context.Context, pid peer.ID, method string, req interface{}, rep interface{}) error
}

type Opts struct {
  Addr string
  Rendezvous string
  Local bool
  PrivKey crypto.PrivKey
  PubKey crypto.PubKey
  OnlyRPC bool
  ConnectOnDiscover bool
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
  errChan chan error
  pubChan map[string] chan<- proto.Message
  protocol protocol.ID
  server *rpc.Server
  l *log.Sublog
}

func New(opts *Opts, ctx context.Context, logger *log.Sublog) (Interface, error) {
  pid := libp2p.Identity(opts.PrivKey)

  // Initialize base pubsub infra
	h, err := libp2p.New(libp2p.ListenAddrStrings(opts.Addr), libp2p.PrivateNetwork(opts.PSK), pid)
	if err != nil {
    return nil, fmt.Errorf("PubSub host creation failure: %w", err)
	}

  var ps *pubsub.PubSub
  if !opts.OnlyRPC {
    // TODO switch to using pubsub.WithDiscovery
    ps, err = pubsub.NewGossipSub(ctx, h)
    if err != nil {
      return nil, fmt.Errorf("GossipSub creation failure: %w", err)
    }
  }

  // Initialize discovery service for pubsub
	disco := discovery.DHT
	if opts.Local {
		disco = discovery.MDNS
	}
	d := discovery.New(ctx, disco, h, opts.Rendezvous, opts.ConnectOnDiscover, logger)

  s := &Transport{
    opts: opts,
    discovery: d,
    pubsub: ps,
    host: h,
    recvChan: make(chan topic.TopicMsg),
    pubChan: make(map[string] chan<- proto.Message),
    l: logger,
  }

  if !opts.OnlyRPC {
    // Join topics
    opts.Topics = append(opts.Topics)
    for _, t := range(opts.Topics) {
      c, err := topic.NewTopicChannel(ctx, s.recvChan, s.ID(), ps, t, s.errChan)
      if err != nil {
        return nil, fmt.Errorf("failed to join topic %s: %w", t, err)
      }
      logger.Info("Joined topic: %q\n", t)
      s.pubChan[t] = c
    }
  }
  return s, nil
}

func (s *Transport) Register(pid string, srv interface{}) error {
  // RPC server for point-to-point requests
  s.protocol = protocol.ID(pid)
  s.server = rpc.NewServer(s.host, s.protocol)
  return s.server.RegisterName(ServiceName, srv)
}

func (s *Transport) Sign(m proto.Message) ([]byte, error) {
  b, err := proto.Marshal(m)
  if err != nil {
    return nil, fmt.Errorf("sign() marshal error: %w", err)
  }

  if sig, err := s.opts.PrivKey.Sign(b); err != nil {
    return nil, fmt.Errorf("sign() crypto error: %w", err)
  } else {
    return sig, nil
  }
}

func (t *Transport) Verify(m proto.Message, signer string, data []byte) (bool, error) {
  pid, err := peer.Decode(signer)
  if err != nil {
    return false, fmt.Errorf("decode peer ID: %w", err)
  }
  pubk, err := pid.ExtractPublicKey()
  if err != nil {
    return false, fmt.Errorf("Extract public key: %w", err)
  }
  msg, err := proto.Marshal(m)
  if err != nil {
    return false, fmt.Errorf("Marshal proto: %w", err)
  }
  return pubk.Verify(msg, data)
}

func (s *Transport) PubKey() crypto.PubKey {
  return s.opts.PubKey
}

func (s *Transport) OnMessage() <-chan topic.TopicMsg {
  return s.recvChan
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
  connectCtx := ctx
  disco := "local"
  if !s.opts.Local {
    disco = "global"
  }
  if s.opts.ConnectTimeout != 0 {
    s.l.Info("Discovering %s pubsub peers (self ID %v, timeout %v)\n", disco, s.ID(), s.opts.ConnectTimeout)
    ctx2, cancel := context.WithTimeout(ctx, s.opts.ConnectTimeout)
    connectCtx = ctx2
    defer cancel()
  } else {
    s.l.Info("Starting %s discovery (self ID %v, no timeout)\n", disco, s.ID())
  }

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

func (t *Transport) Call(ctx context.Context, pid peer.ID, method string, req proto.Message, rep proto.Message) error {
  c := rpc.NewClient(t.host, t.protocol)
  if err := c.CallContext(ctx, pid, ServiceName, "GetState", req, rep); err != nil {
    return fmt.Errorf("Call(%s, %s, %+v, _) error: %w", pid, method, req, err)
  } 
  return nil
}

func(t *Transport) Stream(ctx context.Context, pid peer.ID, method string, req interface{}, rep interface{}) error {
  c := rpc.NewClient(t.host, t.protocol)
  if err := c.Stream(ctx, pid, ServiceName, method, req, rep); err != nil {
    return fmt.Errorf("Stream(%s, %s, _, _) error: %w", pid, method, err)
  } 
  return nil
}

func (t *Transport) GetPeers() peer.IDSlice {
  return t.host.Peerstore().Peers()
}
func (t *Transport) GetPeerAddresses() []*peer.AddrInfo {
  aa := []*peer.AddrInfo{}
  for _, p := range t.GetPeers() {
    a := t.host.Peerstore().PeerInfo(p)
    aa = append(aa, &a)
  }
  return aa
}
func (t *Transport) AddTempPeer(ai *peer.AddrInfo) {
  t.host.Peerstore().AddAddrs(ai.ID, ai.Addrs, peerstore.TempAddrTTL)
}

func ProtoToPeerAddrInfo(ai *pb.AddrInfo) (*peer.AddrInfo, error) {
	pid, err := peer.Decode(ai.GetId())
	if err != nil {
		return nil, fmt.Errorf("Decode error on id %s: %w", ai.GetId(), err)
	}
	p := &peer.AddrInfo{
		ID:    pid,
		Addrs: []ma.Multiaddr{},
	}
	for _, a := range ai.GetAddrs() {
    if len(p.Addrs) > MaxMultiAddr {
      break
    }
		aa, err := ma.NewMultiaddr(a)
		if err != nil {
			return nil, fmt.Errorf("Error creating AddrInfo from string %s: %w", a, err)
		}
		p.Addrs = append(p.Addrs, aa)
	}
  return p, nil
}
func PeerToProtoAddrInfo(ai *peer.AddrInfo) *pb.AddrInfo {
  a := &pb.AddrInfo{
    Id: ai.ID.Pretty(),
    Addrs: []string{},
  }
  for _, ma := range ai.Addrs {
    if len(a.Addrs) > MaxMultiAddr {
      break
    }
    a.Addrs = append(a.Addrs, ma.String())
  }
  return a
}
