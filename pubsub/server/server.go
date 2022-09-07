// package server implements handlers for peerprint service
package server

import (
  "time"
  "fmt"
  "log"
  "sync"
  "github.com/libp2p/go-libp2p-core/crypto"
  "golang.org/x/exp/maps"
	ma "github.com/multiformats/go-multiaddr"
  pb "github.com/smartin015/peerprint/pubsub/proto"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p/core/peer"
  "github.com/smartin015/peerprint/pubsub/prpc"
  "github.com/smartin015/peerprint/pubsub/raft"
  // pubsub "github.com/libp2p/go-libp2p-pubsub"
  "math/rand"
  "context"
)

const (
  AssignmentTopic = "ASSIGN"
  DefaultTopic = "0"
  MaxPoll = 100
)

type PeerPrint struct {
  p *prpc.PRPC
  l *log.Logger
  raftHost host.Host
  raftPath string
  raft *raft.RaftImpl // Only populated if part of the consensus group
  typ pb.PeerType
  ctx context.Context
  topic string
  leader string
  polling *sync.Cond
  pollResult []*pb.PeerStatus
  trustedPeers map[string]struct{}
  leadershipChange chan struct{}
}

func New(ctx context.Context, p *prpc.PRPC, trustedPeers []string, raftAddr string, raftPath string, pkey crypto.PrivKey, l *log.Logger) *PeerPrint {
  if len(trustedPeers) == 0 {
    panic("server.New(): want len(trustedPeers) > 0: required for bootstrapping")
  }

  tp := make(map[string]struct{})
  for _, p := range(trustedPeers) {
    tp[p] = struct{}{}
  }

  // Init raft host early so we can broadcast its resolved multiaddr
  h, err := libp2p.New(libp2p.ListenAddrStrings(raftAddr), libp2p.Identity(pkey))
  if err != nil {
    panic(err)
  }

  return &PeerPrint{
    p: p,
    l: l,
    raftHost: h,
    raft: nil,
    raftPath: raftPath,
    typ: pb.PeerType_UNKNOWN_PEER_TYPE,
    ctx: ctx,
    topic: "",
    leader: "",
    polling: nil,
    pollResult: nil,
    trustedPeers: tp,
    leadershipChange: make(chan struct{}),
  }
}

func (t *PeerPrint) getLeader() string {
  if t.raft != nil {
    return t.raft.Leader()
  }
  return t.leader
}

func (t *PeerPrint) OnPollPeersRequest(topic string, from string, req *pb.PollPeersRequest) {
  if rand.Float64() < req.Probability {
    if err := t.p.Publish(t.ctx, topic, &pb.PeerStatus{
        Id: "todo", //t.ps.ID(),
        Type: pb.PeerType_UNKNOWN_PEER_TYPE,
        State: pb.PeerState_UNKNOWN_PEER_STATE,
    }); err != nil {
      t.l.Println(fmt.Errorf("Failed to respond to OnPeerPoll: %w", err))
    }
  }
}

func (t *PeerPrint) OnPollPeersResponse(topic string, from string, resp *pb.PollPeersResponse) {
  if t.polling != nil {
    t.pollResult = append(t.pollResult, resp.Status)
    if len(t.pollResult) >= MaxPoll {
      t.polling.Broadcast()
      t.polling = nil
    }
  }
}
func (t *PeerPrint) OnAssignmentRequest(topic string, from string, req *pb.AssignmentRequest) {
  if t.getLeader() == t.p.ID {
    t.l.Printf("OnAssignmentRequest(%s, %s, %+v)\n", topic, from, req)
    t.p.Publish(t.ctx, topic, &pb.AssignmentResponse{
      Id: from,
      Topic: t.topic,
      Type: pb.PeerType_LISTENER,
    })
  }
}
func (t *PeerPrint) OnAssignmentResponse(topic string, from string, resp *pb.AssignmentResponse) {
  if _, ok := t.trustedPeers[from]; !ok {
    return
  }
  if resp.Id == t.p.ID { // This is our assignment
    t.topic = resp.GetTopic()
    t.p.JoinTopic(t.ctx, t.topic)
    t.typ = resp.GetType()
    t.l.Printf("Assigned topic %s, type %v\n", t.topic, t.typ)
    if t.typ == pb.PeerType_ELECTABLE {
      if t.raft != nil {
        panic("TODO garbage collect old raft instance")
      }
			if err := t.raftAddrsRequest(); err != nil {
				panic(err)
			}
			t.l.Println("Sent connection request; waiting for raft addresses to propagate")
			time.Sleep(10*time.Second)

      ri, err := raft.New(t.ctx, t.raftHost, t.raftPath, maps.Keys(t.trustedPeers), &t.leadershipChange)
      if err != nil {
        panic(err)
      }
      t.raft = ri
    }
  }
  // We always set the leader when receiving assignment messages
  // on our topic
  if resp.GetTopic() == t.topic {
    t.leader = resp.GetLeaderId()
    fmt.Printf("New leader:", t.leader)
  }
}
func (t *PeerPrint) OnRaftAddrsRequest(topic string, from string, req *pb.RaftAddrsRequest) {
  if t.typ != pb.PeerType_ELECTABLE {
    return;
  }
	if _, ok := t.trustedPeers[from]; ok && topic == t.topic {
    if err := t.connectToRaftPeer(req.RaftId, req.RaftAddrs); err != nil {
      t.l.Printf("%w", err)
    }
    if err := t.raftAddrsResponse(); err != nil {
      t.l.Printf("%w", err)
    }
  }
}
func (t *PeerPrint) OnRaftAddrsResponse(topic string, from string, resp *pb.RaftAddrsResponse) {
  if t.typ != pb.PeerType_ELECTABLE {
    return;
  }
	if _, ok := t.trustedPeers[from]; ok && topic == t.topic {
    if err := t.connectToRaftPeer(resp.RaftId, resp.RaftAddrs); err != nil {
      t.l.Printf("%w", err)
    }
	}
}

func (t *PeerPrint) connectToRaftPeer(id string, addrs []string) error {
  pid, err := peer.Decode(id)
  if err != nil {
    return fmt.Errorf("Decode error on id %s: %w", id, err)
  }
  p := peer.AddrInfo{
    ID: pid,
    Addrs: []ma.Multiaddr{},
  }
  for _, a := range(addrs) {
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

func (t *PeerPrint) OnSetJobRequest(topic string, from string, req *pb.SetJobRequest) {
  // TODO set job
  if t.getLeader() == t.p.ID {}
}
func (t *PeerPrint) OnDeleteJobRequest(topic string, from string, req *pb.DeleteJobRequest) {
  // TODO delete job
  if t.getLeader() == t.p.ID {}
}
func (t *PeerPrint) OnAcquireJobRequest(topic string, from string, req *pb.AcquireJobRequest) {
  // TODO acquire job
  if t.getLeader() == t.p.ID {}
}
func (t *PeerPrint) OnReleaseJobRequest(topic string, from string, req *pb.ReleaseJobRequest) {
  // TODO release job
  if t.getLeader() == t.p.ID {}
}
func (t *PeerPrint) OnJobMutationResponse(topic string, from string, resp *pb.JobMutationResponse) {
  // TODO update state of jobs table
  if t.getLeader() == from {
  }
}

func (t *PeerPrint) Loop() {
  t.p.RegisterCallbacks(t)

  // Whether or not we're a trusted peer, we need to join the assignment topic.
  // Leader election is also broadcast here.
  if err := t.p.JoinTopic(t.ctx, AssignmentTopic); err != nil {
    panic(err)
  }

	_, amTrusted := t.trustedPeers[t.p.ID]
  if amTrusted {
    t.l.Println("We are a trusted peer; overriding assignment")
    t.OnAssignmentResponse(AssignmentTopic, t.p.ID, &pb.AssignmentResponse{
      Id: t.p.ID,
      Topic: DefaultTopic,
      Type: pb.PeerType_ELECTABLE,
      LeaderId: t.p.ID,
    })

  } else if err := t.requestAssignment(!amTrusted); err != nil {
		panic(err)
	}

  for {
    select {
    case <- t.leadershipChange:
      if t.getLeader() == t.p.ID {
        if err := t.publishLeader(); err != nil {
          panic(err)
        }
      }
    case <- t.ctx.Done():
      return
    }
  }
}

func (t *PeerPrint) publishLeader() error {
  // Peers always scrape leader ID from assignment, even if they aren't
  // the assignee
  return t.p.Publish(t.ctx, AssignmentTopic, &pb.AssignmentResponse{
    Topic: t.topic,
    LeaderId: t.getLeader(),
  })
}

func (t *PeerPrint) publishState() {
  jobs, err := t.raft.GetState()
  if err != nil {
    t.l.Printf("GetState error: %w\n", err)
    return 
  }

  if err = t.p.Publish(t.ctx, t.topic, &pb.JobMutationResponse {
    Jobs: jobs,
  }); err != nil {
    t.l.Printf("Publish error: %w\n", err)
  }
}

func (t *PeerPrint) raftAddrs() []string {
  a := []string{}
  for _, addr := range(t.raftHost.Addrs()) {
    a = append(a, addr.String())
  }
  return a
}

func (t *PeerPrint) raftAddrsRequest() error {
	return t.p.Publish(t.ctx, t.topic, &pb.RaftAddrsRequest{
      RaftId: t.raftHost.ID().Pretty(),
      RaftAddrs: t.raftAddrs(),
	})
}
func (t *PeerPrint) raftAddrsResponse() error {
	return t.p.Publish(t.ctx, t.topic, &pb.RaftAddrsResponse{
      RaftId: t.raftHost.ID().Pretty(),
      RaftAddrs: t.raftAddrs(),
	})
}

func (t *PeerPrint) requestAssignment(repeat bool) error {
  for t.topic == "" {
    if err := t.p.Publish(t.ctx, AssignmentTopic, &pb.AssignmentRequest{
    }); err != nil {
      return err
    }
    time.Sleep(10*time.Second)
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
