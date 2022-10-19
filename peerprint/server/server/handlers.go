// package server implements handlers for peerprint service
package server

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
	"github.com/smartin015/peerprint/peerprint_server/raft"
	"golang.org/x/exp/maps"
	"time"
	// pubsub "github.com/libp2p/go-libp2p-pubsub"
	"math/rand"
)

func (t *PeerPrint) Handle(topic string, peer string, p proto.Message) (proto.Message, error) {
	switch v := p.(type) {
	// proto/peers.proto
  case *pb.SelfStatusRequest:
    return t.OnSelfStatusRequest(topic, peer, v)
	case *pb.AssignmentRequest:
		return t.OnAssignmentRequest(topic, peer, v)
	case *pb.PollPeersRequest:
		return t.OnPollPeersRequest(topic, peer, v)
	case *pb.RaftAddrsRequest:
		return t.OnRaftAddrsRequest(topic, peer, v)
	case *pb.AssignmentResponse:
		return nil, t.OnAssignmentResponse(topic, peer, v)
	case *pb.PollPeersResponse:
		return nil, t.OnPollPeersResponse(topic, peer, v)
	case *pb.RaftAddrsResponse:
		return nil, t.OnRaftAddrsResponse(topic, peer, v)

	// proto/jobs.proto
	case *pb.GetJobsRequest:
		return t.OnGetJobsRequest(topic, peer, v)
	case *pb.SetJobRequest:
		return t.OnSetJobRequest(topic, peer, v)
	case *pb.DeleteJobsRequest:
		return t.OnDeleteJobsRequest(topic, peer, v)
	case *pb.AcquireJobRequest:
		return t.OnAcquireJobRequest(topic, peer, v)
	case *pb.ReleaseJobRequest:
		return t.OnReleaseJobRequest(topic, peer, v)
	case *pb.State:
		return nil, t.OnState(topic, peer, v)

	default:
		return nil, fmt.Errorf("Received unknown type response on topic")
	}
}


func (t *PeerPrint) OnSelfStatusRequest(topic string, from string, req *pb.SelfStatusRequest) (*pb.PeerStatus, error) {
  return &pb.PeerStatus{
    Id:    t.p.ID,
    Topic: t.topic,
    Type:  t.typ,
    State: pb.PeerState_UNKNOWN_PEER_STATE,
  }, nil
}

func (t *PeerPrint) OnPollPeersRequest(topic string, from string, req *pb.PollPeersRequest) (*pb.PeerStatus, error) {
	if rand.Float64() < req.Probability {
		return &pb.PeerStatus{
			Id:    "todo", //t.ps.ID(),
			Type:  pb.PeerType_UNKNOWN_PEER_TYPE,
			State: pb.PeerState_UNKNOWN_PEER_STATE,
		}, nil
	}
  return nil, nil
}

func (t *PeerPrint) OnPollPeersResponse(topic string, from string, resp *pb.PollPeersResponse) error {
	if t.polling != nil {
		t.pollResult = append(t.pollResult, resp.Status)
		if len(t.pollResult) >= MaxPoll {
			t.polling.Broadcast()
			t.polling = nil
		}
	}
  return nil
}
func (t *PeerPrint) OnAssignmentRequest(topic string, from string, req *pb.AssignmentRequest) (*pb.AssignmentResponse, error) {
	if t.getLeader() == t.p.ID {
		return &pb.AssignmentResponse{
			Id:    from,
			Topic: t.topic,
			Type:  pb.PeerType_LISTENER,
		}, nil
	}
  return nil, nil
}
func (t *PeerPrint) OnAssignmentResponse(topic string, from string, resp *pb.AssignmentResponse) error {
	if _, ok := t.trustedPeers[from]; !ok {
		return fmt.Errorf("got OnAssignmentResponse from untrusted peer %s", from)
	}
	if resp.Id == t.p.ID { // This is our assignment
		t.topic = resp.GetTopic()
		t.p.JoinTopic(t.ctx, t.topic, t.l)
		t.typ = resp.GetType()
		if t.typ == pb.PeerType_ELECTABLE {
			if t.raft != nil {
				return fmt.Errorf("TODO garbage collect old raft instance")
			}
			if err := t.p.Publish(t.ctx, t.topic, t.raftAddrsRequest()); err != nil {
				return err
			}
			t.l.Println("Sent connection request; waiting for raft addresses to propagate")
			time.Sleep(10 * time.Second)

			ri, err := raft.New(t.ctx, t.raftHost, t.raftPath, maps.Keys(t.trustedPeers), &t.leadershipChange, t.l)
			if err != nil {
				return err
			}
			t.raft = ri
			t.state.Set(t.raft, nil) // Switch to raft for state handling
		}
	}
	// We always set the leader when receiving assignment messages
	// on our topic
	if resp.GetTopic() == t.topic {
		t.leader = resp.GetLeaderId()
		t.l.Printf("New leader:", t.leader)
	}

  return nil
}

func (t *PeerPrint) OnRaftAddrsRequest(topic string, from string, req *pb.RaftAddrsRequest) (*pb.RaftAddrsResponse, error) {
	if t.typ != pb.PeerType_ELECTABLE {
		return nil, nil // Not our problem
	}
	if _, ok := t.trustedPeers[from]; ok && topic == t.topic {
		if err := t.connectToRaftPeer(req.RaftId, req.RaftAddrs); err != nil {
      return nil, err
		}
		return t.raftAddrsResponse(), nil
	}
  return nil, nil
}

func (t *PeerPrint) OnRaftAddrsResponse(topic string, from string, resp *pb.RaftAddrsResponse) error {
	if t.typ != pb.PeerType_ELECTABLE {
		return nil // Not our problem
	}
	if _, ok := t.trustedPeers[from]; ok && topic == t.topic {
		return t.connectToRaftPeer(resp.RaftId, resp.RaftAddrs)
	}
  return nil
}

func (t *PeerPrint) OnGetJobsRequest(topic string, from string, req *pb.GetJobsRequest) (*pb.State, error) {
  return t.resolveState()
}

func (t *PeerPrint) OnSetJobRequest(topic string, from string, req *pb.SetJobRequest) (*pb.State, error) {
	if t.getLeader() != t.p.ID {
    return nil, fmt.Errorf("OnSetJobRequest from non-leader %w ignored", from)
	}
	s, err := t.state.Get()
	if err != nil {
		return nil, fmt.Errorf("state.Get(): %w\n", err)
	}
  j := s.Jobs[req.GetJob().GetId()]
  ln := j.GetLock().GetPeer()
  if ln != "" && ln != req.GetJob().GetLock().GetPeer() {
    return nil, fmt.Errorf("Rejecting SetJob request: lock data was tampered with")
  }

  s.Jobs[req.GetJob().GetId()] = req.GetJob()
	return t.commitAndGetState(s)
}

func (t *PeerPrint) OnDeleteJobsRequest(topic string, from string, req *pb.DeleteJobsRequest) (*pb.State, error) {
	if t.getLeader() != t.p.ID {
		return nil, fmt.Errorf("OnDeleteJobsRequest from non-leader %w ignored", from)
	}
	s, err := t.state.Get()
	if err != nil {
		return nil, fmt.Errorf("state.Get(): %w\n", err)
	}
	//if _, ok := s.Jobs[req.GetJobId()]; !ok {
	//	return fmt.Errorf("Job %s not found\n", req.GetJobId())
	//}
	//delete(s.Jobs, req.GetJobId())
	return t.commitAndGetState(s)
}

func (t *PeerPrint) OnAcquireJobRequest(topic string, from string, req *pb.AcquireJobRequest) (*pb.State, error) {
	if t.getLeader() != t.p.ID {
		return nil, fmt.Errorf("OnAcquireJobRequest from non-leader %w ignored", from)
	}
	s, err := t.state.Get()
	if err != nil {
		return nil, fmt.Errorf("state.Get(): %w\n", err)
	}
	//j, ok := s.Jobs[req.GetJobId()]
	//if !ok {
	//	return fmt.Errorf("Job %s not found\n", req.GetJobId())
	//}
	//expiry := uint64(time.Now().Unix() - 2*60*60) // TODO constant
	//if j.Lock.GetPeer() == "" || j.Lock.Created < expiry {
	//	j.Lock = &pb.Lock{
	//		Peer:    from,
	//		Created: uint64(time.Now().Unix()),
	//	}
	//}
	return t.commitAndGetState(s)
}

func (t *PeerPrint) OnReleaseJobRequest(topic string, from string, req *pb.ReleaseJobRequest) (*pb.State, error) {
	if t.getLeader() != t.p.ID {
		return nil, fmt.Errorf("OnReleaseJobRequest from non-leader %w ignored", from)
	}
	s, err := t.state.Get()
	if err != nil {
		return nil, fmt.Errorf("state.Get(): %w\n", err)
	}
	//j, ok := s.Jobs[req.GetJobId()]
	//if !ok {
	//	return fmt.Errorf("Job %s not found\n", req.GetJobId())
	//}
	//expiry := uint64(time.Now().Unix() - 2*60*60) // TODO constant
	//if j.Lock.GetPeer() == from || j.Lock.Created < expiry {
	//	j.Lock = nil
	//}
	return t.commitAndGetState(s)
}
func (t *PeerPrint) OnState(topic string, from string, resp *pb.State) error {
	if t.getLeader() == from && t.raft == nil {
		t.state.Set(nil, resp)
	}
  return nil
}
