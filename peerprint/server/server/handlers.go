// package server implements handlers for peerprint service
package server

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
	"time"
	"math/rand"
)

func (t *Server) isSelfLeader() bool {
  return t.getLeader() == t.getID()
}

func (t *Server) isLeader(peer string) bool {
  return t.getLeader() == peer
}

func (t *Server) isTrusted(peer string) bool {
  if _, ok := t.trustedPeers[peer]; ok {
    return true
  }
  return t.isLeader(peer)
}

func (t *Server) isSelfElectable() bool {
  return t.getType() == pb.PeerType_ELECTABLE
}

func (t *Server) CanHandleMessage(from string, p proto.Message) bool {
  switch v := p.(type) {
  case *pb.SetJobRequest:
    return t.isSelfLeader()
  case *pb.DeleteJobRequest:
    return t.isSelfLeader()
  case *pb.AcquireJobRequest:
    return t.isSelfLeader()
  case *pb.ReleaseJobRequest:
    return t.isSelfLeader()
  case *pb.State:
    return t.isLeader(from)
  case *pb.Leader:
    return t.isTrusted(from)
  case *pb.PeersSummary:
    return t.isLeader(from)
  case *pb.AssignmentRequest:
    return t.isSelfLeader()
  case *pb.AssignmentResponse:
    // Always require a trusted peer, and only observe messages sent to us
    return t.isTrusted(from) && v.GetId() == t.getID()
  case *pb.RaftAddrsRequest:
    return t.isTrusted(from)
  case *pb.RaftAddrsResponse:
    return t.isSelfElectable() && t.isTrusted(from)
  default:
    t.l.Println("No handler validation for message type")
    return false // Fail closed so we're forced to implement
	}
}

func (t *Server) Handle(topic string, peer string, p proto.Message) (proto.Message, error) {
  t.l.Println("Handling ", p.ProtoReflect().Descriptor().FullName())
  if !t.CanHandleMessage(peer, p) {
    return nil, nil
  }

	switch v := p.(type) {
	// proto/peers.proto
	case *pb.AssignmentRequest:
		return t.OnAssignmentRequest(topic, peer, v)
	case *pb.PollPeersRequest:
		return t.OnPollPeersRequest(topic, peer, v)
	case *pb.RaftAddrsRequest:
		return t.OnRaftAddrsRequest(topic, peer, v)
	case *pb.AssignmentResponse:
		return t.OnAssignmentResponse(topic, peer, v)
	case *pb.PollPeersResponse:
		return nil, t.OnPollPeersResponse(topic, peer, v)
  case *pb.PeersSummary:
    return nil, t.OnPeersSummary(topic, peer, v)
	case *pb.RaftAddrsResponse:
		return nil, t.OnRaftAddrsResponse(topic, peer, v)

	// proto/jobs.proto
	case *pb.SetJobRequest:
		return t.OnSetJobRequest(topic, peer, v)
	case *pb.DeleteJobRequest:
		return t.OnDeleteJobRequest(topic, peer, v)
	case *pb.AcquireJobRequest:
		return t.OnAcquireJobRequest(topic, peer, v)
	case *pb.ReleaseJobRequest:
		return t.OnReleaseJobRequest(topic, peer, v)
	case *pb.State:
		return nil, t.OnState(topic, peer, v)

	default:
		return nil, fmt.Errorf("No handler matching message %+v on topic %s", p, topic)
	}
}

func (t *Server) OnPollPeersRequest(topic string, from string, req *pb.PollPeersRequest) (*pb.PollPeersResponse, error) {
	if rand.Float64() < req.Probability {
		return &pb.PollPeersResponse{
      Status: &t.status,
    }, nil
	}
  return nil, nil
}

func (t *Server) OnPollPeersResponse(topic string, from string, resp *pb.PollPeersResponse) error {
  t.poller.Update(resp.Status)
  return nil
}
func (t *Server) OnAssignmentRequest(topic string, from string, req *pb.AssignmentRequest) (*pb.AssignmentResponse, error) {
  return &pb.AssignmentResponse{
    LeaderId: t.getLeader(),
    Id:    from,
    Topic: t.getTopic(),
    Type:  pb.PeerType_LISTENER,
  }, nil
}

func (t *Server) OnAssignmentResponse(topic string, from string, resp *pb.AssignmentResponse) (*pb.RaftAddrsRequest, error) {
  // New assignment, so close other topic
  if prev, ok := t.sendPubsub[t.getTopic()]; ok {
    close(prev)
  }

  t.status.Topic = resp.GetTopic()
  t.status.Type = resp.GetType()
	t.raft.SetLeader(resp.GetLeaderId())

  // Open the new topic if we haven't already
  if _, ok := t.sendPubsub[t.getTopic()]; !ok {
    pub, err := t.open(t.getTopic())
    if err != nil {
      return nil, err
    }
    t.sendPubsub[t.getTopic()] = pub
  }

  // Begin raft connection process if we're electable, otherwise wait for
  // state to arrive over pubsub (no action)
  if t.getType() == pb.PeerType_ELECTABLE {
    return t.raftAddrsRequest(), nil
  }

  select {
  case t.roleAssigned<- t.getType():
  default:
  }
  return nil, nil
}

func (t *Server) OnRaftAddrsRequest(topic string, from string, req *pb.RaftAddrsRequest) (*pb.RaftAddrsResponse, error) {
	if _, ok := t.trustedPeers[from]; ok && topic == t.getTopic() {
    peers := t.raft.GetPeers()
    peers = append(peers, req.AddrInfo)
    if err := t.raft.SetPeers(peers); err != nil {
      return nil, err
    }
		return t.raftAddrsResponse(), nil
	}
  return nil, nil
}

func (t *Server) OnRaftAddrsResponse(topic string, from string, resp *pb.RaftAddrsResponse) error {
  // TODO handle duplicate peers
  return t.raft.SetPeers(resp.GetPeers())
}

func (t *Server) OnSetJobRequest(topic string, from string, req *pb.SetJobRequest) (*pb.State, error) {
	s, err := t.raft.Get()
  if err != nil || s == nil {
		return nil, fmt.Errorf("raft.Get(): %w\n", err)
	}
  // Confirm job can be written to if already exists
  j, ok := s.Jobs[req.GetJob().GetId()]
  if ok {
    if err := t.checkMutable(j, from); err != nil {
      return nil, err
    }
  }

  // Ensure lock data is not tampered with
  if j != nil {
    req.GetJob().Lock = j.GetLock()
  }

  s.Jobs[req.GetJob().GetId()] = req.GetJob()
	return t.raft.Commit(s)
}

func (t *Server) OnDeleteJobRequest(topic string, from string, req *pb.DeleteJobRequest) (*pb.State, error) {
  s, _, err := t.getMutableJob(req.GetId(), from)
	if err != nil {
		return nil, fmt.Errorf("OnDeleteJobRequest: %w", err)
	}
  delete(s.Jobs, req.GetId())
	return t.raft.Commit(s)
}

func (t *Server) OnAcquireJobRequest(topic string, from string, req *pb.AcquireJobRequest) (*pb.State, error) {
  s, j, err := t.getMutableJob(req.GetId(), from)
	if err != nil {
		return nil, fmt.Errorf("OnAcquireJobRequest: %w", err)
	}
  j.Lock = &pb.Lock {
    Peer:    from,
    Created: uint64(time.Now().Unix()),
  }
	return t.raft.Commit(s)
}

func (t *Server) OnReleaseJobRequest(topic string, from string, req *pb.ReleaseJobRequest) (*pb.State, error) {
  s, j, err := t.getMutableJob(req.GetId(), from)
	if err != nil {
		return nil, fmt.Errorf("OnReleaseJobRequest: %w", err)
	}
	j.Lock = nil
	return t.raft.Commit(s)
}
func (t *Server) OnState(topic string, from string, resp *pb.State) error {
  t.pushCmd <- resp
	if t.getLeader() == from {
    _, err := t.raft.Commit(resp)
    return err
	}
  return nil
}
func (t *Server) OnLeader(topic string, from string, resp *pb.Leader) error {
  t.raft.SetLeader(resp.GetId())
  return nil
}

func (t *Server) OnPeersSummary(topic string, from string, resp *pb.PeersSummary) error {
  // Peer summary is only "stored" ephemerally, i.e. passed along to the 
  // wrapper
  t.pushCmd <- resp
  return nil
}
