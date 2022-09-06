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
)

type PeerPrint struct {
  p *prpc.PRPC
  l *log.Logger
  typ pb.PeerType
  ctx context.Context
}

func New(ctx context.Context, p *prpc.PRPC, l *log.Logger) *PeerPrint {
  return &PeerPrint{
    p: p,
    typ: pb.PeerType_UNKNOWN_PEER_TYPE,
    ctx: ctx,
  }
}

func (t *PeerPrint) OnPollPeersRequest(topic string, req *pb.PollPeersRequest) {
  if rand.Float64() < req.Probability {
    if err := t.p.Publish(t.ctx, req.Topic, &pb.PeerStatus{
        Id: "todo", //t.ps.ID(),
        Type: pb.PeerType_UNKNOWN_PEER_TYPE,
        State: pb.PeerState_UNKNOWN_PEER_STATE,
    }); err != nil {
      t.l.Println(fmt.Errorf("Failed to respond to OnPeerPoll: %w", err))
    }
  }
}

func (t *PeerPrint) OnPollPeersResponse(topic string, req *pb.PollPeersResponse) {}
func (t *PeerPrint) OnAssignmentRequest(topic string, req *pb.AssignmentRequest) {}
func (t *PeerPrint) OnAssignmentResponse(topic string, req *pb.AssignmentResponse) {}
func (t *PeerPrint) OnSetJobRequest(topic string, req *pb.SetJobRequest) {}
func (t *PeerPrint) OnDeleteJobRequest(topic string, req *pb.DeleteJobRequest) {}
func (t *PeerPrint) OnAcquireJobRequest(topic string, req *pb.AcquireJobRequest) {}
func (t *PeerPrint) OnReleaseJobRequest(topic string, req *pb.ReleaseJobRequest) {}
func (t *PeerPrint) OnJobMutationResponse(topic string, req *pb.JobMutationResponse) {}
func (t *PeerPrint) OnGetJobsRequest(topic string, req *pb.GetJobsRequest) {}
func (t *PeerPrint) OnGetJobsResponse(topic string, req *pb.GetJobsResponse) {}


func (t *PeerPrint) Loop() {
  t.p.RegisterCallbacks(t)
  t.p.Publish(t.ctx, AssignmentTopic, &pb.AssignmentRequest{})
  select {
  case <- t.ctx.Done():
    return
  }
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
