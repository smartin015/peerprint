// package server implements handlers for peerprint service
package server

import (
  "fmt"
  "log"
  pb "github.com/smartin015/peerprint/pubsub/proto"
  "github.com/smartin015/peerprint/pubsub/prpc"
  // pubsub "github.com/libp2p/go-libp2p-pubsub"
  "math/rand"
  "context"
)

const (
  AssignmentTopic = "ASSIGN"
  MaxPoll = 100
)

type PeerPrint struct {
  p *prpc.PRPC
  l *log.Logger
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
    typ: pb.PeerType_UNKNOWN_PEER_TYPE,
    ctx: ctx,
    trustedPeers: tp,
  }
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
  if t.leader == t.p.ID {
    t.p.Publish(t.ctx, topic, &pb.AssignmentResponse{
      Id: peer,
      Topic: t.topic,
    })
  }
}
func (t *PeerPrint) OnAssignmentResponse(topic string, peer string, resp *pb.AssignmentResponse) {
  if _, ok := t.trustedPeers[peer]; ok && resp.Id == t.p.ID {
    t.topic = resp.GetTopic()
    t.p.JoinTopic(t.ctx, t.topic)
  }
}
func (t *PeerPrint) OnSetJobRequest(topic string, peer string, req *pb.SetJobRequest) {
  // TODO set job
  if t.leader == t.p.ID {}
}
func (t *PeerPrint) OnDeleteJobRequest(topic string, peer string, req *pb.DeleteJobRequest) {
  // TODO delete job
  if t.leader == t.p.ID {}
}
func (t *PeerPrint) OnAcquireJobRequest(topic string, peer string, req *pb.AcquireJobRequest) {
  // TODO acquire job
  if t.leader == t.p.ID {}
}
func (t *PeerPrint) OnReleaseJobRequest(topic string, peer string, req *pb.ReleaseJobRequest) {
  // TODO release job
  if t.leader == t.p.ID {}
}
func (t *PeerPrint) OnJobMutationResponse(topic string, peer string, resp *pb.JobMutationResponse) {
  // TODO update state of jobs table
  if t.leader == peer {}
}
func (t *PeerPrint) OnGetJobsRequest(topic string, peer string, req *pb.GetJobsRequest) {
  // TODO 
  if t.leader == t.p.ID {}
}
func (t *PeerPrint) OnGetJobsResponse(topic string, peer string, resp *pb.GetJobsResponse) {
  if t.leader == peer {}
}


func (t *PeerPrint) Loop() {
  t.p.RegisterCallbacks(t)
  t.requestAssignment()
  select {
  case <- t.ctx.Done():
    return
  }
}

func (t *PeerPrint) requestAssignment() {
	t.p.JoinTopic(t.ctx, AssignmentTopic)
  t.p.Publish(t.ctx, AssignmentTopic, &pb.AssignmentRequest{})
}

func (t *PeerPrint) PollPeers(topic string, prob float64) error {
  return t.p.Publish(t.ctx, topic, &pb.PollPeersRequest{
    Probability: prob,
  })
}

/*
func (t *PeerPrint) SetPeerType(req *pb.SetPeerTypeRequest) (*pb.SetPeerTypeResponse, error) {return nil, nil}
func (t *PeerPrint) DoElection(*pb.DoElectionRequest) (*pb.GetConsensusStatusResponse, error) {return nil, nil}
func (t *PeerPrint) GetConsensusStatus(*pb.GetConsensusStatusRequest) (*pb.GetConsensusStatusResponse, error) {return nil, nil}
func (t *PeerPrint) MintToken(*pb.MintTokenRequest) (*pb.MintTokenResponse, error) {return nil, nil}
func (t *PeerPrint) SetJob(*pb.SetJobRequest) (*pb.SetJobResponse, error) {return nil, nil}
func (t *PeerPrint) DeleteJob(*pb.DeleteJobRequest) (*pb.DeleteJobResponse, error) {return nil, nil}
func (t *PeerPrint) AcquireJob(*pb.AcquireJobRequest) (*pb.AcquireJobResponse, error) {return nil, nil}
func (t *PeerPrint) ReleaseJob(*pb.ReleaseJobRequest) (*pb.ReleaseJobResponse, error) {return nil, nil}
func (t *PeerPrint) GetJobs(*pb.GetJobsRequest) (*pb.GetJobsResponse, error) {return nil, nil}
func (t *PeerPrint) StatJobs(*pb.StatJobsRequest) (*pb.StatJobsResponse, error) {return nil, nil}
*/
