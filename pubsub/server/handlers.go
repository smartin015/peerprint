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

func (t *PeerPrint) OnPollPeersRequest(topic string, from string, req *pb.PollPeersRequest) {
	if rand.Float64() < req.Probability {
		if err := t.p.Publish(t.ctx, topic, &pb.PeerStatus{
			Id:    "todo", //t.ps.ID(),
			Type:  pb.PeerType_UNKNOWN_PEER_TYPE,
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
			Id:    from,
			Topic: t.topic,
			Type:  pb.PeerType_LISTENER,
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
			time.Sleep(10 * time.Second)

			ri, err := raft.New(t.ctx, t.raftHost, t.raftPath, maps.Keys(t.trustedPeers), &t.leadershipChange)
			if err != nil {
				panic(err)
			}
			t.raft = ri
			t.state.Set(t.raft, nil) // Switch to raft for state handling
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
		return
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
		return
	}
	if _, ok := t.trustedPeers[from]; ok && topic == t.topic {
		if err := t.connectToRaftPeer(resp.RaftId, resp.RaftAddrs); err != nil {
			t.l.Printf("%w", err)
		}
	}
}

func (t *PeerPrint) commitAndPublish(s *pb.State) {
  if err := t.raft.Commit(s); err != nil {
		fmt.Printf("raft.Commit(): %w\n", err)
	}
	t.publishState()
}

func (t *PeerPrint) OnSetJobRequest(topic string, from string, req *pb.SetJobRequest) {
	if t.getLeader() != t.p.ID {
		return
	}
	s, err := t.state.Get()
	if err != nil {
		fmt.Printf("state.Get(): %w\n", err)
		return
	}
	s.Jobs[req.GetJob().GetId()] = req.GetJob()
	t.commitAndPublish(s)
}

func (t *PeerPrint) OnDeleteJobRequest(topic string, from string, req *pb.DeleteJobRequest) {
	if t.getLeader() != t.p.ID {
		return
	}
	s, err := t.state.Get()
	if err != nil {
		fmt.Printf("state.Get(): %w\n", err)
		return
	}
	if _, ok := s.Jobs[req.GetJobId()]; !ok {
		fmt.Printf("Job %s not found\n", req.GetJobId())
		return
	}
	delete(s.Jobs, req.GetJobId())
	t.commitAndPublish(s)
}

func (t *PeerPrint) OnAcquireJobRequest(topic string, from string, req *pb.AcquireJobRequest) {
	if t.getLeader() != t.p.ID {
		return
	}
	s, err := t.state.Get()
	if err != nil {
		fmt.Printf("state.Get(): %w\n", err)
		return
	}
	j, ok := s.Jobs[req.GetJobId()]
	if !ok {
		fmt.Printf("Job %s not found\n", req.GetJobId())
		return
	}
	expiry := uint64(time.Now().Unix() - 2*60*60) // TODO constant
	if j.Lock.GetPeer() == "" || j.Lock.Created < expiry {
		j.Lock = &pb.Lock{
			Peer:    from,
			Created: uint64(time.Now().Unix()),
		}
	}
	t.commitAndPublish(s)
}

func (t *PeerPrint) OnReleaseJobRequest(topic string, from string, req *pb.ReleaseJobRequest) {
	if t.getLeader() != t.p.ID {
		return
	}
	s, err := t.state.Get()
	if err != nil {
		fmt.Printf("state.Get(): %w\n", err)
		return
	}
	j, ok := s.Jobs[req.GetJobId()]
	if !ok {
		fmt.Printf("Job %s not found\n", req.GetJobId())
		return
	}
	expiry := uint64(time.Now().Unix() - 2*60*60) // TODO constant
	if j.Lock.GetPeer() == from || j.Lock.Created < expiry {
		j.Lock = nil
	}
	t.commitAndPublish(s)
}
func (t *PeerPrint) OnState(topic string, from string, resp *pb.State) {
	if t.getLeader() == from && t.raft == nil {
		t.state.Set(nil, resp)
	}
}
