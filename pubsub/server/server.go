// package server implements handlers for peerprint service
package server

import (
  "time"
  "fmt"
  "log"
  pb "github.com/smartin015/peerprint/pubsub/proto"
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
  ri *raft.RaftImpl // Only populated if part of the consensus group
  typ pb.PeerType
  ctx context.Context
  topic string
  leader string
  polling bool
  pollResult []*pb.PeerStatus
  trustedPeers map[string]struct{}
}

func New(ctx context.Context, p *prpc.PRPC, trustedPeers []string, l *log.Logger) *PeerPrint {
  if len(trustedPeers) == 0 {
    panic("server.New(): want len(trustedPeers) > 0: required for bootstrapping")
  }

  tp := make(map[string]struct{})
  for _, p := range(trustedPeers) {
    tp[p] = struct{}{}
  }

  return &PeerPrint{
    p: p,
    l: l,
    ri: nil,
    typ: pb.PeerType_UNKNOWN_PEER_TYPE,
    ctx: ctx,
    topic: "",
    leader: "",
    polling: false,
    pollResult: nil,
    trustedPeers: tp,
  }
}

func (t *PeerPrint) getLeader() string {
  if t.ri != nil {
    return t.ri.Leader()
  }
  return t.leader
}

func (t *PeerPrint) OnPollPeersRequest(topic string, peer string, req *pb.PollPeersRequest) {
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

func (t *PeerPrint) OnPollPeersResponse(topic string, peer string, resp *pb.PollPeersResponse) {
  if t.polling {
    //t.pollResult = append(resp.Status)
    //if len(pollResult) >= MaxPoll {
    //  t.polling(false)
    //}
  }
}
func (t *PeerPrint) OnAssignmentRequest(topic string, peer string, req *pb.AssignmentRequest) {
  if t.getLeader() == t.p.ID {
    t.l.Printf("OnAssignmentRequest(%s, %s, %+v)\n", topic, peer, req)
    t.p.Publish(t.ctx, topic, &pb.AssignmentResponse{
      Id: peer,
      Topic: t.topic,
      // Always start off as a follower, although it's possible to be promoted later
      Type: pyb.PeerType_FOLLOWER,
    })
  }
}
func (t *PeerPrint) OnAssignmentResponse(topic string, peer string, resp *pb.AssignmentResponse) {
  if _, ok := t.trustedPeers[peer]; ok && resp.Id == t.p.ID {
    t.topic = resp.GetTopic()
    t.p.JoinTopic(t.ctx, t.topic)
    t.typ = resp.GetType()
    t.leader = resp.GetLeader()
    t.l.Printf("Assigned topic %s, type %v\n", t.topic, t.typ)
    if t.typ == pb.PeerType_ELECTABLE {
      if t.raft != nil {
        panic("TODO garbage  collect old raft instance")
      }

      cg := []peer.AddrInfo{}
      for _, c := range(resp.GetConsensusGroup() {
        fmt.Println("TODO convert to AddrInfo")
      }
      t.raft = raft.New(t.raftAddr, t.raftPath, cg)
    }
  }
}
func (t *PeerPrint) OnSetJobRequest(topic string, peer string, req *pb.SetJobRequest) {
  // TODO set job
  if t.getLeader() == t.p.ID {}
}
func (t *PeerPrint) OnDeleteJobRequest(topic string, peer string, req *pb.DeleteJobRequest) {
  // TODO delete job
  if t.getLeader() == t.p.ID {}
}
func (t *PeerPrint) OnAcquireJobRequest(topic string, peer string, req *pb.AcquireJobRequest) {
  // TODO acquire job
  if t.getLeader() == t.p.ID {}
}
func (t *PeerPrint) OnReleaseJobRequest(topic string, peer string, req *pb.ReleaseJobRequest) {
  // TODO release job
  if t.getLeader() == t.p.ID {}
}
func (t *PeerPrint) OnJobsResponse(topic string, peer string, resp *pb.JobMutationResponse) {
  // TODO update state of jobs table
  if t.getLeader() == peer {
  }
}
func (t *PeerPrint) OnGetJobsRequest(topic string, peer string, req *pb.GetJobsRequest) {
  if t.getLeader() == t.p.ID {
    t.publishState()
  }
}
func (t *PeerPrint) OnNewLeaderResponse(topic string, peer string, resp *pb.GetJobsResponse) {
  if _, ok := t.trustedPeers[peer]; ok {
    t.leader = resp.Leader
    t.l.Printf("New leader %s\n", t.leader)
  }
}

func (t *PeerPrint) Loop() {
  t.p.RegisterCallbacks(t)

  if _, ok := t.trustedPeers[t.p.ID]; ok {
    t.l.Println("We are a trusted peer; overriding assignment")
    t.OnAssignmentResponse(AssignmentTopic, t.p.ID, &pb.AssignmentResponse{
      Topic: DefaultTopic,
      Type: pb.PeerType_TRUSTED,
      ID: t.p.ID,
      Leader: "",
    })
  } else {
    t.l.Println("Requesting assignment")
    if err := t.requestAssignment(); err != nil {
      panic(err)
    }
  }
  select {
  case <- t.ri.newLeader:
    if t.getLeader() == t.ID {
      if err := publishLeader(); err != nil {
        panic(err)
      }
    }
  case <- t.ctx.Done():
    return
  }
}

func (t *PeerPrint) publishLeader() error {
  return t.p.Publish(t.ctx, t.topic, &pb.LeaderResponse{})
}

func (t *PeerPrint) publishState() {
  jobs, err := t.raft.GetState()
  if err != nil {
    t.l.Printf("GetState error: %w\n", err)
    return 
  }

  if err = t.p.Publish(t.ctx, t.topic, &pb.JobsResponse {
    Jobs: jobs,
  }); err != nil {
    t.l.Printf("Publish error: %w\n", err)
  }
}

func (t *PeerPrint) requestAssignment() error {
  if err := t.p.JoinTopic(t.ctx, AssignmentTopic); err != nil {
    return err
  }
  for t.topic == "" {
    if err := t.p.Publish(t.ctx, AssignmentTopic, &pb.AssignmentRequest{}); err != nil {
      return err
    }
    time.Sleep(10*time.Second)
  }
}

func (t *PeerPrint) PollPeers(topic string, prob float64) error {
  return t.p.Publish(t.ctx, topic, &pb.PollPeersRequest{
    Probability: prob,
  })
}
