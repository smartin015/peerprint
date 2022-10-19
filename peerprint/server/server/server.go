// package server implements handlers for peerprint service
package server

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"
	ma "github.com/multiformats/go-multiaddr"
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
	"github.com/smartin015/peerprint/peerprint_server/prpc"
	"github.com/smartin015/peerprint/peerprint_server/raft"
	"github.com/smartin015/peerprint/peerprint_server/cmd"
	"log"
	"sync"
	"time"
)

const (
	AssignmentTopic = "ASSIGN"
	DefaultTopic    = "0"
	MaxPoll         = 100
  LockTimeoutSeconds = 2*60*60
  PollTimeout = 60 * time.Second
)

type PeerPrint struct {
	p                *prpc.PRPC
	l                *log.Logger
	raftHost         host.Host
	raftPath         string
	raft             *raft.RaftImpl // Only populated if part of the consensus group
	state            *ServerState
  cmd             *cmd.Zmq
	typ              pb.PeerType
	ctx              context.Context
	topic            string
	leader           string
	polling          *sync.Cond
  bootstrap        bool
	pollResult       []*pb.PeerStatus
  peersSummary      *pb.PeersSummary
	trustedPeers     map[string]struct{}
	leadershipChange chan struct{}
}

type PeerPrintOptions struct {
  Ctx context.Context
  Prpc *prpc.PRPC
  TrustedPeers []string
  RaftAddr string
  RaftPath string
  ZmqServerAddr string
  ZmqPushAddr string
  Bootstrap bool
  PKey crypto.PrivKey
  Logger *log.Logger
}

func New(opts PeerPrintOptions) *PeerPrint {
	if len(opts.TrustedPeers) == 0 {
		panic("server.New(): want len(opts.TrustedPeers) > 0: required for bootstrapping")
	}

	tp := make(map[string]struct{})
	for _, p := range opts.TrustedPeers {
		tp[p] = struct{}{}
	}

	// Init raft host early so we can broadcast its resolved multiaddr
	h, err := libp2p.New(libp2p.ListenAddrStrings(opts.RaftAddr), libp2p.Identity(opts.PKey))
	if err != nil {
		panic(err)
	}

  var z *cmd.Zmq
  if opts.ZmqServerAddr != "" {
    z = cmd.New(opts.ZmqServerAddr, opts.ZmqPushAddr)
    opts.Logger.Println("ZMQ server at ", opts.ZmqServerAddr)
  } else {
    opts.Logger.Println("No zmq addr given, skipping zmq init")
  }

  return &PeerPrint{
		p:                opts.Prpc,
		l:                opts.Logger,
		raftHost:         h,
		raft:             nil,
		raftPath:         opts.RaftPath,
    cmd:              z,
		state:            NewServerState(),
		typ:              pb.PeerType_UNKNOWN_PEER_TYPE,
		ctx:              opts.Ctx,
		topic:            "",
		leader:           "",
    bootstrap:        opts.Bootstrap,
		polling:          nil,
		pollResult:       nil,
		trustedPeers:     tp,
		leadershipChange: make(chan struct{}),
	}
}

func (t *PeerPrint) getLeader() string {
	if t.raft != nil {
		return t.raft.Leader()
	}
	return t.leader
}

func (t *PeerPrint) connectToRaftPeer(id string, addrs []string) error {
	pid, err := peer.Decode(id)
	if err != nil {
		return fmt.Errorf("Decode error on id %s: %w", id, err)
	}
	p := peer.AddrInfo{
		ID:    pid,
		Addrs: []ma.Multiaddr{},
	}
	for _, a := range addrs {
		aa, err := ma.NewMultiaddr(a)
		if err != nil {
			return fmt.Errorf("Error creating AddrInfo from string %s: %w", a, err)
		}
		p.Addrs = append(p.Addrs, aa)
	}
	if err := t.raftHost.Connect(t.ctx, p); err != nil {
		return fmt.Errorf("RAFT peer connection error: %w", err)
	}
	t.l.Println("Connected to RAFT peer", id)
	return nil
}

func (t *PeerPrint) onCmd(req proto.Message) {
  var err error
  var rep proto.Message
  // Skip pubsub if we're wrapping the leader, which is authoritative
  if t.p.ID == t.getLeader() {
    rep, err = t.Handle(t.topic, t.p.ID, req)
    if rep != nil {
      err = t.p.Publish(t.ctx, t.topic, rep)
    }
  } else {
    // Otherwise we publish the command as-is, return OK
    err = t.p.Publish(t.ctx, t.topic, req)
  }

  if err != nil {
    t.cmd.Send(&pb.Error{Status: err.Error()})
  } else {
    t.cmd.Send(&pb.Ok{})
  }
}

func (t *PeerPrint) Loop() {
	t.p.RegisterCallback(t.Handle)

  if t.cmd != nil {
    go t.cmd.Loop(t.onCmd)
  }

	// Whether or not we're a trusted peer, we need to join the assignment topic.
	// Leader election is also broadcast here.
	if err := t.p.JoinTopic(t.ctx, AssignmentTopic, t.l); err != nil {
		panic(err)
	}

	_, amTrusted := t.trustedPeers[t.p.ID]
	if amTrusted {
		t.l.Println("We are a trusted peer; overriding assignment")
		t.OnAssignmentResponse(AssignmentTopic, t.p.ID, &pb.AssignmentResponse{
			Id:       t.p.ID,
			Topic:    DefaultTopic,
			Type:     pb.PeerType_ELECTABLE,
			LeaderId: "",
		})

	} else if err := t.requestAssignment(!amTrusted); err != nil {
		panic(err)
	}

  pollTicker := time.NewTicker(PollTimeout)
  pollTicker.Stop()

	for {
		select {
		case <-t.leadershipChange:
			if t.getLeader() == t.p.ID {
				if err := t.publishLeader(); err != nil {
					panic(err)
				}
        // Important to publish state after initial leader election,
        // so all listener nodes are also on the same page.
        // This may fail on new queue that hasn't been bootstrapped yet
        s, err := t.resolveState()
        if err != nil {
          panic(err)
        }
        if err := t.p.Publish(t.ctx, t.topic, s); err != nil {
          t.l.Println(err)
        }
        if err := t.cmd.Push(s); err != nil {
          t.l.Println(err)
        }
        pollTicker.Reset(5 * time.Second)
			}
    case <-pollTicker.C:
      if t.polling != nil {
        t.l.Println("Already polling, skipping poll")
      } else {
        ctx2, _ := context.WithTimeout(t.ctx, 5 * time.Second)
        go t.pollPeersSync(ctx2, t.topic)
        pollTicker.Reset(PollTimeout)
      }
		case <-t.ctx.Done():
			return
		}
	}
}

func (t *PeerPrint) publishLeader() error {
	// Peers always scrape leader ID from assignment, even if they aren't
	// the assignee
	return t.p.Publish(t.ctx, AssignmentTopic, &pb.AssignmentResponse{
		Topic:    t.topic,
		LeaderId: t.getLeader(),
	})
}

func (t *PeerPrint) resolveState() (*pb.State, error) {
	s, err := t.state.Get()
	if err != nil {
    if err == raft.ErrNoState && t.bootstrap {
      t.l.Println("RAFT state not bootstrapped; doing so now")
      s, err = t.raft.BootstrapState()
      if err != nil {
        return nil, fmt.Errorf("bootstrap error: %w", err)
      }
    } else {
		  return nil, fmt.Errorf("state.Get() error: %w (did you bootstrap?)\n", err)
    }
	}
  t.bootstrap = false
	return s, nil
}

func (t *PeerPrint) commitAndGetState(s *pb.State) (*pb.State, error) {
  if err := t.raft.Commit(s); err != nil {
    return nil, fmt.Errorf("raft.Commit(): %w\n", err)
  }
  return t.resolveState()
}

func (t *PeerPrint) raftAddrs() []string {
	a := []string{}
	for _, addr := range t.raftHost.Addrs() {
		a = append(a, addr.String())
	}
	return a
}

func (t *PeerPrint) raftAddrsRequest() *pb.RaftAddrsRequest {
	return &pb.RaftAddrsRequest{
		RaftId:    t.raftHost.ID().Pretty(),
		RaftAddrs: t.raftAddrs(),
	}
}
func (t *PeerPrint) raftAddrsResponse() *pb.RaftAddrsResponse {
	return &pb.RaftAddrsResponse{
		RaftId:    t.raftHost.ID().Pretty(),
		RaftAddrs: t.raftAddrs(),
	}
}

func (t *PeerPrint) getMutableJob(jid string, peer string) (*pb.State, *pb.Job, error) {
	s, err := t.state.Get()
	if err != nil {
		return nil, nil, fmt.Errorf("state.Get(): %w", err)
	}
	j, ok := s.Jobs[jid]
	if !ok {
		return nil, nil, fmt.Errorf("Job %s not found", jid)
	}
	expiry := uint64(time.Now().Unix() - LockTimeoutSeconds)
  p := j.Lock.GetPeer()
	if !(p == "" || p == peer || j.Lock.Created < expiry) {
    return nil, nil, fmt.Errorf("Cannot modify job %s; acquired by %s", jid, j.Lock.GetPeer())
  }
  return s, j, nil
}

func (t *PeerPrint) peerStatus() *pb.PeerStatus {
  return &pb.PeerStatus{
    Id:    t.p.ID,
    Topic: t.topic,
    Type:  t.typ,
    Leader: t.getLeader(),
    State: pb.PeerState_UNKNOWN_PEER_STATE,
  }
}

func (t *PeerPrint) requestAssignment(repeat bool) error {
	for t.topic == "" {
		if err := t.p.Publish(t.ctx, AssignmentTopic, &pb.AssignmentRequest{}); err != nil {
			return err
		}
		time.Sleep(10 * time.Second)
	}
	return nil
}

func (t *PeerPrint) pollPeersSync(ctx context.Context, topic string) error {
  prob := 0.9 // TODO adjust dynamically based on last poll performance
  t.l.Println("Doing periodic peer poll - probability", prob)
	t.pollResult = nil
  l := &sync.Mutex{}
  l.Lock()
	t.polling = sync.NewCond(l)
  defer func() { t.polling = nil }()
	if err := t.p.Publish(t.ctx, topic, &pb.PollPeersRequest{
		Probability: prob,
	}); err != nil {
		return err
	}

	await := make(chan bool)
	go func() {
		t.polling.Wait()
		await <- true
	}()
	select {
	case <-await:
	case <-ctx.Done():
	}

  // Assuming the received number of peers is the mode - most commonly occurring value,
  // len(pollResult) = floor((pop + 1)*prob)
  pop := int64(float64(len(t.pollResult)) / prob)
  t.peersSummary = &pb.PeersSummary{
    PeerEstimate: pop,
    Variance: float64(pop) * prob * (1 - prob),
  }
  t.l.Println("Poll summary:", t.peersSummary)
  t.p.Publish(t.ctx, topic, t.peersSummary)
  if err := t.cmd.Push(t.peersSummary); err != nil {
    t.l.Println(err)
  }
	return nil
}
