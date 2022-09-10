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
	pb "github.com/smartin015/peerprint/pubsub/proto"
	"github.com/smartin015/peerprint/pubsub/prpc"
	"github.com/smartin015/peerprint/pubsub/raft"
	"github.com/smartin015/peerprint/pubsub/cmd"
	"log"
	"sync"
	"time"
)

const (
	AssignmentTopic = "ASSIGN"
	DefaultTopic    = "0"
	MaxPoll         = 100
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
	trustedPeers     map[string]struct{}
	leadershipChange chan struct{}
}

func New(ctx context.Context, p *prpc.PRPC, trustedPeers []string, raftAddr string, raftPath string, zmqServerAddr string, bootstrap bool, pkey crypto.PrivKey, l *log.Logger) *PeerPrint {
	if len(trustedPeers) == 0 {
		panic("server.New(): want len(trustedPeers) > 0: required for bootstrapping")
	}

	tp := make(map[string]struct{})
	for _, p := range trustedPeers {
		tp[p] = struct{}{}
	}

	// Init raft host early so we can broadcast its resolved multiaddr
	h, err := libp2p.New(libp2p.ListenAddrStrings(raftAddr), libp2p.Identity(pkey))
	if err != nil {
		panic(err)
	}

  var z *cmd.Zmq
  if zmqServerAddr != "" {
    z = cmd.New(zmqServerAddr)
    fmt.Println("ZMQ server at ", zmqServerAddr)
  } else {
    fmt.Println("No zmq addr given, skipping zmq init")
  }

  return &PeerPrint{
		p:                p,
		l:                l,
		raftHost:         h,
		raft:             nil,
		raftPath:         raftPath,
    cmd:              z,
		state:            NewServerState(),
		typ:              pb.PeerType_UNKNOWN_PEER_TYPE,
		ctx:              ctx,
		topic:            "",
		leader:           "",
    bootstrap:        bootstrap,
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
	fmt.Println("Connected to RAFT peer", id)
	return nil
}

func (t *PeerPrint) sendErr(e error) {
  t.cmd.Send(&pb.Error{Status: e.Error()})
}

func (t *PeerPrint) sendWithLoopback(req proto.Message) error {
  if t.getLeader() == t.p.ID {
    return t.Recv(t.topic, t.p.ID, req)
  } else {
    return t.p.Publish(t.ctx, t.topic, req)
  }
}

func (t *PeerPrint) onCmd(req proto.Message) {
  err := t.sendWithLoopback(req)
  if err != nil {
    t.sendErr(err)
  } else {
    s, err := t.state.Get()
    if err != nil {
      t.sendErr(fmt.Errorf("Failed to get state: %w", err))
    } else {
      t.cmd.Send(s)
    }
  }
}

func (t *PeerPrint) Loop() {
	t.p.RegisterCallback(t.Recv)

  if t.cmd != nil {
    go t.cmd.Loop(t.onCmd)
  }

	// Whether or not we're a trusted peer, we need to join the assignment topic.
	// Leader election is also broadcast here.
	if err := t.p.JoinTopic(t.ctx, AssignmentTopic); err != nil {
		panic(err)
	}

	_, amTrusted := t.trustedPeers[t.p.ID]
	if amTrusted {
		t.l.Println("We are a trusted peer; overriding assignment")
		t.OnAssignmentResponse(AssignmentTopic, t.p.ID, &pb.AssignmentResponse{
			Id:       t.p.ID,
			Topic:    DefaultTopic,
			Type:     pb.PeerType_ELECTABLE,
			LeaderId: t.p.ID,
		})

	} else if err := t.requestAssignment(!amTrusted); err != nil {
		panic(err)
	}

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
        if err := t.publishState(); err != nil {
          t.l.Println(err)
        }
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

func (t *PeerPrint) publishState() error {
	s, err := t.state.Get()
	if err != nil {
    if err == raft.ErrNoState && t.bootstrap {
      t.l.Println("RAFT state not bootstrapped; doing so now")
      s, err = t.raft.BootstrapState()
      if err != nil {
        return fmt.Errorf("bootstrap error: %w", err)
      }
    } else {
		  return fmt.Errorf("state.Get() error: %w\n", err)
    }
	}
  t.bootstrap = false

	if err = t.p.Publish(t.ctx, t.topic, s); err != nil {
		return fmt.Errorf("Publish error: %w\n", err)
	}
  return nil
}

func (t *PeerPrint) raftAddrs() []string {
	a := []string{}
	for _, addr := range t.raftHost.Addrs() {
		a = append(a, addr.String())
	}
	return a
}

func (t *PeerPrint) raftAddrsRequest() error {
	return t.p.Publish(t.ctx, t.topic, &pb.RaftAddrsRequest{
		RaftId:    t.raftHost.ID().Pretty(),
		RaftAddrs: t.raftAddrs(),
	})
}
func (t *PeerPrint) raftAddrsResponse() error {
	return t.p.Publish(t.ctx, t.topic, &pb.RaftAddrsResponse{
		RaftId:    t.raftHost.ID().Pretty(),
		RaftAddrs: t.raftAddrs(),
	})
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

func (t *PeerPrint) pollPeersSync(ctx context.Context, topic string, prob float64) error {
	t.pollResult = nil
	t.polling = sync.NewCond(&sync.Mutex{})
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
	t.polling = nil
	return nil
}
