package raft

import (
  "io"
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/raft"
	consensus "github.com/libp2p/go-libp2p-consensus"
	p2praft "github.com/libp2p/go-libp2p-raft"
	"github.com/libp2p/go-libp2p/core/host"
	pb "github.com/smartin015/peerprint/pubsub/proto"
  "google.golang.org/protobuf/proto"
)

// MarshableState implements the interface given at 
// https://github.com/libp2p/go-libp2p-raft/blob/8c69d1ffe0db4c78b7f15129ed433280d8347f4f/consensus.go
// Rather than using the default "Msgpack" library for serialiation, we rely on 
// the go protobuf marshal/unmarshal methods. This works around weird serialization errors
// that seem to occur when protobuf objects contain maps.
type MarshableState struct {
  State *pb.State
}

func (rs *MarshableState) Marshal(out io.Writer) error {
  m, err := proto.Marshal(rs.State)
  if err != nil {
    return err
  }
  _, err = out.Write(m)
  return err
}

func (rs *MarshableState) Unmarshal(in io.Reader) error {
  b, err := io.ReadAll(in)
  if err != nil {
    return err
  }
  return proto.Unmarshal(b, rs.State)
}

type RaftImpl struct {
	host      host.Host
	raft      *raft.Raft
	consensus consensus.Consensus
	transport *raft.NetworkTransport
	actor     *p2praft.Actor
	leaderObs *raft.Observer
	newLeader *chan struct{}
}

func (ri *RaftImpl) WaitForLeader(ctx context.Context) error {
	obsCh := make(chan raft.Observation, 1)
	observer := raft.NewObserver(obsCh, false, nil)
	ri.raft.RegisterObserver(observer)
	defer ri.raft.DeregisterObserver(observer)

	ticker := time.NewTicker(time.Second / 2)
	defer ticker.Stop()
	for {
		select {
		case obs := <-obsCh:
			switch obs.Data.(type) {
			case raft.RaftState:
				if ri.raft.Leader() != "" {
					return nil
				}
			}
		case <-ticker.C:
			if ri.raft.Leader() != "" {
				return nil
			}
		case <-ctx.Done():
			return fmt.Errorf("WaitForLeader context timed out")
		}
	}
}

func (ri *RaftImpl) Shutdown() {
	defer ri.transport.Close()
	defer ri.host.Close()
	defer ri.raft.DeregisterObserver(ri.leaderObs)
	err := ri.raft.Shutdown().Error()
	if err != nil {
		panic(err)
	}
}

// New creates a libp2p host and transport layer, a new consensus, FSM, actor, config, logstore, snapshot method, and
// basically all other moving parts needed to elect a leader and synchronize a log across the leader peers.
// The result has an .Observer field channel which provides updates
func New(ctx context.Context, h host.Host, filestorePath string, peers []string, newLeader *chan struct{}) (*RaftImpl, error) {
	if len(peers) == 0 {
		return nil, fmt.Errorf("No peers in raft group - cannot create")
	}

	transport, err := p2praft.NewLibp2pTransport(h, 2*time.Second)
	if err != nil {
		return nil, err
	}
	consensus := p2praft.NewConsensus(&MarshableState{
    State: &pb.State{Jobs: nil},
  })
	servers := make([]raft.Server, len(peers))
	for i, peer := range peers {
		servers[i] = raft.Server{
			Suffrage: raft.Voter,
			ID:       raft.ServerID(peer),
			Address:  raft.ServerAddress(peer), // Libp2pTransport treats ID as address here
		}
	}

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(h.ID().Pretty())
	snapshots, err := raft.NewFileSnapshotStore(filestorePath, 3, nil)
	if err != nil {
		return nil, err
	}
	// InmemStore implements the LogStore and StableStore interface. It should NOT EVER be used for production. It is used only for unit tests. Use the MDBStore implementation instead.
	logStore := raft.NewInmemStore()

	bootstrapped, err := raft.HasExistingState(logStore, logStore, snapshots)
	if err != nil {
		return nil, err
	}
	if !bootstrapped {
		raft.BootstrapCluster(config, logStore, logStore, snapshots, transport, raft.Configuration{Servers: servers})
	} else {
		fmt.Println("Cluster already initialized")
	}

	r, err := raft.NewRaft(config, consensus.FSM(), logStore, logStore, snapshots, transport)
	if err != nil {
		return nil, err
	}
	actor := p2praft.NewActor(r)
	consensus.SetActor(actor)

	ri := &RaftImpl{
		host:      h,
		raft:      r,
		consensus: consensus,
		transport: transport,
		actor:     actor,
		newLeader: newLeader,
	}
	go ri.observeLeadership(context.Background())
	return ri, nil
}

func (ri *RaftImpl) observeLeadership(ctx context.Context) {
	last := ""
	ticker := time.NewTicker(5 * time.Second)
	obsCh := make(chan raft.Observation, 1)
	observer := raft.NewObserver(obsCh, false, nil)
	ri.raft.RegisterObserver(observer)
	defer ri.raft.DeregisterObserver(observer)
	defer ticker.Stop()
	for {
		select {
		case <-obsCh:
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
		if l := ri.Leader(); l != last {
			last = l
			*ri.newLeader <- struct{}{}
		}
	}
}

func (ri *RaftImpl) Leader() string {
	return string(ri.raft.Leader())
}

func (ri *RaftImpl) Commit(s *pb.State) error {
	if ri.actor.IsLeader() {
    agreedState, err := ri.consensus.CommitState(&MarshableState{State: s}) // Blocking
		if err != nil {
			return err
		}
		if agreedState == nil {
			return fmt.Errorf("agreedState is nil: commited on a non-leader?")
		}
		return nil
	} else {
		return fmt.Errorf("Not leader; cannot commit")
	}
}

func (ri *RaftImpl) BootstrapState() (*pb.State, error) {
  s := &pb.State{
    Jobs: make(map[string]*pb.Job),
  }
  j := &pb.Job{Id: "testid", Protocol: "testing", Data: []byte{1,2,3}}
  s.Jobs[j.GetId()] = j

  if err := ri.Commit(s); err != nil {
    return nil, err
  }
  return ri.GetState()
}

var ErrNoState = p2praft.ErrNoState

func (ri *RaftImpl) GetState() (*pb.State, error) {
	s, err := ri.consensus.GetCurrentState()
	if err != nil {
		return nil, err
	}
	rs, ok := s.(*MarshableState)
	if !ok {
		return nil, fmt.Errorf("State type assertion failed")
	}
	return rs.State, nil
}
