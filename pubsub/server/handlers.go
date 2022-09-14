// package server implements handlers for peerprint service
package server

import (
	"fmt"
	pb "github.com/smartin015/peerprint/pubsub/proto"
	"github.com/smartin015/peerprint/pubsub/raft"
	"golang.org/x/exp/maps"
	"time"
	// pubsub "github.com/libp2p/go-libp2p-pubsub"
	"math/rand"
)

func (t *PeerPrint) Recv(topic string, peer string, resp interface{}) error {
	switch v := resp.(type) {
	// proto/peers.proto
	case *pb.AssignmentRequest:
		return t.OnAssignmentRequest(topic, peer, v)
	case *pb.AssignmentResponse:
		return t.OnAssignmentResponse(topic, peer, v)
	case *pb.PollPeersRequest:
		return t.OnPollPeersRequest(topic, peer, v)
	case *pb.PollPeersResponse:
		return t.OnPollPeersResponse(topic, peer, v)
	case *pb.RaftAddrsRequest:
		return t.OnRaftAddrsRequest(topic, peer, v)
	case *pb.RaftAddrsResponse:
		return t.OnRaftAddrsResponse(topic, peer, v)

	// proto/jobs.proto
	case *pb.GetJobsRequest:
		return t.OnGetJobsRequest(topic, peer, v)
	case *pb.SetJobRequest:
		return t.OnSetJobRequest(topic, peer, v)
	case *pb.DeleteJobRequest:
		return t.OnDeleteJobRequest(topic, peer, v)
	case *pb.AcquireJobRequest:
		return t.OnAcquireJobRequest(topic, peer, v)
	case *pb.ReleaseJobRequest:
		return t.OnReleaseJobRequest(topic, peer, v)
	case *pb.State:
		return t.OnState(topic, peer, v)

	default:
		return fmt.Errorf("Received unknown type response on topic")
	}
}

func (t *PeerPrint) OnPollPeersRequest(topic string, from string, req *pb.PollPeersRequest) error {
	if rand.Float64() < req.Probability {
		return t.p.Publish(t.ctx, topic, &pb.PeerStatus{
			Id:    "todo", //t.ps.ID(),
			Type:  pb.PeerType_UNKNOWN_PEER_TYPE,
			State: pb.PeerState_UNKNOWN_PEER_STATE,
		})
	}
  return nil
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
func (t *PeerPrint) OnAssignmentRequest(topic string, from string, req *pb.AssignmentRequest) error {
	if t.getLeader() == t.p.ID {
		return t.p.Publish(t.ctx, topic, &pb.AssignmentResponse{
			Id:    from,
			Topic: t.topic,
			Type:  pb.PeerType_LISTENER,
		})
	}
  return nil
}
func (t *PeerPrint) OnAssignmentResponse(topic string, from string, resp *pb.AssignmentResponse) error {
	if _, ok := t.trustedPeers[from]; !ok {
		return fmt.Errorf("got OnAsisgnmentResponse from untrusted peer %s", from)
	}
	if resp.Id == t.p.ID { // This is our assignment
		t.topic = resp.GetTopic()
		t.p.JoinTopic(t.ctx, t.topic, t.l)
		t.typ = resp.GetType()
		if t.typ == pb.PeerType_ELECTABLE {
			if t.raft != nil {
				return fmt.Errorf("TODO garbage collect old raft instance")
			}
			if err := t.raftAddrsRequest(); err != nil {
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

func (t *PeerPrint) OnRaftAddrsRequest(topic string, from string, req *pb.RaftAddrsRequest) error {
	if t.typ != pb.PeerType_ELECTABLE {
		return nil // Not our problem
	}
	if _, ok := t.trustedPeers[from]; ok && topic == t.topic {
		if err := t.connectToRaftPeer(req.RaftId, req.RaftAddrs); err != nil {
      return err
		}
		return t.raftAddrsResponse()
	}
  return nil
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

func (t *PeerPrint) commitAndPublish(s *pb.State) error {
  if err := t.raft.Commit(s); err != nil {
		return fmt.Errorf("raft.Commit(): %w\n", err)
	}
	return t.publishState()
}

func (t *PeerPrint) OnGetJobsRequest(topic string, from string, req *pb.GetJobsRequest) error {
  return t.publishState()
}

func (t *PeerPrint) OnSetJobRequest(topic string, from string, req *pb.SetJobRequest) error {
	if t.getLeader() != t.p.ID {
    return fmt.Errorf("OnSetJobRequest from non-leader %w ignored", from)
	}
	s, err := t.state.Get()
	if err != nil {
		return fmt.Errorf("state.Get(): %w\n", err)
	}
  j := s.Jobs[req.GetJob().GetId()]
  ln := j.GetLock().GetPeer()
  if ln != "" && ln != req.GetJob().GetLock().GetPeer() {
    return fmt.Errorf("Rejecting SetJob request: lock data was tampered with")
  }

  s.Jobs[req.GetJob().GetId()] = req.GetJob()
	return t.commitAndPublish(s)
}

func (t *PeerPrint) OnDeleteJobRequest(topic string, from string, req *pb.DeleteJobRequest) error {
	if t.getLeader() != t.p.ID {
		return fmt.Errorf("OnDeleteJobRequest from non-leader %w ignored", from)
	}
	s, err := t.state.Get()
	if err != nil {
		return fmt.Errorf("state.Get(): %w\n", err)
	}
	//if _, ok := s.Jobs[req.GetJobId()]; !ok {
	//	return fmt.Errorf("Job %s not found\n", req.GetJobId())
	//}
	//delete(s.Jobs, req.GetJobId())
	return t.commitAndPublish(s)
}

func (t *PeerPrint) OnAcquireJobRequest(topic string, from string, req *pb.AcquireJobRequest) error {
	if t.getLeader() != t.p.ID {
		return fmt.Errorf("OnAcquireJobRequest from non-leader %w ignored", from)
	}
	s, err := t.state.Get()
	if err != nil {
		return fmt.Errorf("state.Get(): %w\n", err)
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
	return t.commitAndPublish(s)
}

func (t *PeerPrint) OnReleaseJobRequest(topic string, from string, req *pb.ReleaseJobRequest) error {
	if t.getLeader() != t.p.ID {
		return fmt.Errorf("OnReleaseJobRequest from non-leader %w ignored", from)
	}
	s, err := t.state.Get()
	if err != nil {
		return fmt.Errorf("state.Get(): %w\n", err)
	}
	//j, ok := s.Jobs[req.GetJobId()]
	//if !ok {
	//	return fmt.Errorf("Job %s not found\n", req.GetJobId())
	//}
	//expiry := uint64(time.Now().Unix() - 2*60*60) // TODO constant
	//if j.Lock.GetPeer() == from || j.Lock.Created < expiry {
	//	j.Lock = nil
	//}
	return t.commitAndPublish(s)
}
func (t *PeerPrint) OnState(topic string, from string, resp *pb.State) error {
	if t.getLeader() == from && t.raft == nil {
		t.state.Set(nil, resp)
	}
  return nil
}
