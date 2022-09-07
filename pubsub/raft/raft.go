package raftimpl

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

  pb "github.com/smartin015/peerprint/pubsub/proto"
	libp2p "github.com/libp2p/go-libp2p"
  p2praft "github.com/libp2p/go-libp2p-raft"
	consensus "github.com/libp2p/go-libp2p-consensus"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"

	"github.com/hashicorp/raft"
)

type raftState struct {
  Jobs []*pb.Job
}

type RaftImpl struct {
  host *host.Host
  raft *raft.Raft
  consensus *consensus.Consensus
  transport *raft.NetworkTransport
  actor *p2praft.Actor
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
	err := ri.raft.Shutdown().Error()
	if err != nil {
		panic(err)
	}
}

// New creates a libp2p host and transport layer, a new consensus, FSM, actor, config, logstore, snapshot method, and 
// basically all other moving parts needed to elect a leader and synchronize a log across the leader peers.
// The result has an .Observer field channel which provides updates
func New(addr string, filestorePath string, peers []peer.AddrInfo) (*RaftImpl, error)  {
  h, err := libp2p.New(
    libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/9997", listenPort)),
  )
  if err != nil {
    return nil, err
  }
	transport, err := NewLibp2pTransport(h, 2*time.Second)
	if err != nil {
		return nil, err
	}
  consensus := consensus.NewConsensus(&raftState{Jobs: nil})
	servers := make([]raft.Server, len(peers))
  for i, peer := range(peers) {
	  h.Peerstore().AddAddrs(peer.ID(), peer.Addrs(), peerstore.PermanentAddrTTL)
		servers[i] = raft.Server{
			Suffrage: raft.Voter,
			ID:       raft.ServerID(peer.ID.Pretty()),
			Address:  raft.ServerAddress(peer.ID.Pretty()),
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
		fmt.Println("Already initialized!!")
	}

	raft, err := raft.NewRaft(config, consensus.FSM(), logStore, logStore, snapshots, transport)
	if err != nil {
		return nil, err
	}
	actor := p2praft.NewActor(raft)
	consensus.SetActor(actor)

  return &RaftImpl {
    host: h,
    raft: raft,
    consensus: consensus,
    transport: transport,
    actor: actor,
  }, nil
}

func (ri *RaftImpl) UpdatePeerList(peers []*peer.AddrInfo) error {
  return fmt.Errorf("Unimplemented")
}

func (ri *RaftImpl) Leader() bool {
  return ri.actor.IsLeader()
}

func (ri *RaftImpl) Commit(jobs []*pb.Jobs) error {
	if ri.actor.IsLeader() {
		agreedState, err := ri.consensus.CommitState(&raftState{
      Jobs: jobs,
    }) // Blocking
		if err != nil {
      return err
		}
		if agreedState == nil {
			return fmt.Errorf("agreedState is nil: commited on a non-leader?")
		}
    return nil
	} else {
    return fm.Errorf("Not leader; cannot commit")
  }
}

func (ri *RaftImpl) GetState() ([]*pb.Jobs, error) {
	s, err := ri.consensus.GetCurrentState()
	if err != nil {
		return nil, err
	}
  rs, ok := s.(*raftState)
  if !ok {
    return nil, fmt.Errorf("raftState type assertion failed")
  }
  return rs.Jobs, nil
}
