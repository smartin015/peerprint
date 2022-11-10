package raft

import (
  "io"
	"context"
  "log"
	"fmt"
	"time"
  "github.com/libp2p/go-libp2p/core/peer"
  ma "github.com/multiformats/go-multiaddr"
	"github.com/hashicorp/raft"
	p2praft "github.com/libp2p/go-libp2p-raft"
	"github.com/libp2p/go-libp2p/core/host"
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
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

type Raft interface{
  AddrInfo() *pb.AddrInfo
  Connect(*pb.AddrInfo) error
  GetPeers() []*pb.AddrInfo
  HasPeer(*pb.AddrInfo) bool

  Leader() string
  SetLeader(string)
  LeaderChan() (chan struct{})
  Commit(*pb.State) (*pb.State, error)
  Get() (*pb.State, error)
}

type LocalMemoryImpl struct {
  s *pb.State
  l string
  lc chan struct{}
}
func NewInMemory() *LocalMemoryImpl {
  return &LocalMemoryImpl{
    s: &pb.State{Jobs: make(map[string]*pb.Job)},
    l: "",
    lc: make(chan struct{}),
  }
}
func (ri *LocalMemoryImpl) AddrInfo() *pb.AddrInfo { return nil }
func (ri *LocalMemoryImpl) Connect(*pb.AddrInfo) error {
  return nil
}
func (ri *LocalMemoryImpl) GetPeers() []*pb.AddrInfo {
  return nil
}
func (ri *LocalMemoryImpl) Leader() string {
  return ri.l
}
func (ri *LocalMemoryImpl) LeaderChan() (chan struct{}) {
  return ri.lc
}
func (ri *LocalMemoryImpl) Get() (*pb.State, error) {
  return ri.s, nil
}
func (ri *LocalMemoryImpl) Commit(s *pb.State) (*pb.State, error) {
  ri.s = s
  return ri.s, nil
}
func (ri *LocalMemoryImpl) SetLeader(l string) {
  ri.l = l
}


type RaftImpl struct {
  ctx       context.Context
	host      host.Host
  filestorePath string
  logger *log.Logger
  peers   []*pb.AddrInfo
  servers []raft.Server

	raft      *raft.Raft
	consensus *p2praft.Consensus
	transport *raft.NetworkTransport
	actor     *p2praft.Actor
	leaderObs *raft.Observer
	newLeader chan struct{}
}

func (ri *RaftImpl) AddrInfo() *pb.AddrInfo {
	a := []string{}
	for _, addr := range ri.host.Addrs() {
		a = append(a, addr.String())
	}

  return &pb.AddrInfo {
    Id: ri.host.ID().Pretty(),
    Addrs: a,
  }
}

func (ri *RaftImpl) SetLeader(leader string) {
  // Do nothing - raft leadership is earned, not assigned
}
func (ri *RaftImpl) LeaderChan() (chan struct{}) {
  return ri.newLeader
}

func (ri *RaftImpl) Shutdown() {
  ri.logger.Println("raft.Shutdown() called")
	defer ri.transport.Close()
	defer ri.host.Close()
	defer ri.raft.DeregisterObserver(ri.leaderObs)
	err := ri.raft.Shutdown().Error()
	if err != nil {
		panic(err)
	}
}

func New(ctx context.Context, h host.Host, filestorePath string, l *log.Logger) *RaftImpl {
  ri := &RaftImpl{
    ctx: ctx,
    host: h,
    filestorePath: filestorePath,
    logger: l,
    servers: []raft.Server{},
    peers: []*pb.AddrInfo{},
    transport: nil, // Signals that we haven't yet initialized raft
    newLeader: make(chan struct{}, 5),
  }

  // Add ourselves as a server (i.e. part of the electorate)
  ai := ri.AddrInfo()
  ri.servers = append(ri.servers, raft.Server{
          Suffrage: raft.Voter,
          ID:       raft.ServerID(ai.GetId()),
          Address:  raft.ServerAddress(ai.GetId()), // Libp2pTransport treats ID as address here
  })
  return ri
}

func protoToPeerAddrInfo(ai *pb.AddrInfo) (*peer.AddrInfo, error) {
	pid, err := peer.Decode(ai.GetId())
	if err != nil {
		return nil, fmt.Errorf("Decode error on id %s: %w", ai.GetId(), err)
	}
	p := &peer.AddrInfo{
		ID:    pid,
		Addrs: []ma.Multiaddr{},
	}
	for _, a := range ai.GetAddrs() {
		aa, err := ma.NewMultiaddr(a)
		if err != nil {
			return nil, fmt.Errorf("Error creating AddrInfo from string %s: %w", a, err)
		}
		p.Addrs = append(p.Addrs, aa)
	}
  return p, nil
}

func (ri *RaftImpl) addServer(ai *pb.AddrInfo) error {
  s := raft.Server{
			Suffrage: raft.Voter,
			ID:       raft.ServerID(ai.GetId()),
			Address:  raft.ServerAddress(ai.GetId()), // Libp2pTransport treats ID as address here
	}

  if !ri.HasPeer(ai) {
    ri.peers = append(ri.peers, ai)
    ri.servers = append(ri.servers, s)
  }

  p, err := protoToPeerAddrInfo(ai)
  if err != nil {
    return fmt.Errorf("addServer() failed to convert pb.AddrInfo to peer.AddrInfo: %w", err)
  }

  if err = ri.host.Connect(ri.ctx, *p); err != nil {
    return fmt.Errorf("addServer() RAFT peer connection error: %w", err)
  }

  if ri.raft != nil && ri.raft.State() == raft.Leader {
    // Calling AddVoter with an existing voter updates its address
    // https://pkg.go.dev/github.com/hashicorp/raft#Raft.AddVoter
    ri.logger.Println("AddVoter()", s.ID)
    // TODO handle failure to add due to raft config index changing
    ri.raft.AddVoter(s.ID, s.Address, ri.raft.AppliedIndex(), 0)
  }
	return nil
}

func (ri *RaftImpl) GetPeers() []*pb.AddrInfo {
  return ri.peers
}

func (ri *RaftImpl) HasPeer(ai *pb.AddrInfo) bool {
  for _, ei := range ri.peers {
    if ai.GetId() == ei.GetId() {
      return true
    }
  }
  return false
}

func (ri *RaftImpl) Connect(ai *pb.AddrInfo) error {
  ri.logger.Println("Connect(): ", ai)
  if err := ri.addServer(ai); err != nil {
    return err
  }
  if ri.transport == nil {
    return ri.setup()
  }
  return nil
}


func (ri *RaftImpl) setup() error {
	if len(ri.servers) == 0 {
		return fmt.Errorf("No servers in raft group - cannot connect")
	}
  var err error
  if ri.transport, err = p2praft.NewLibp2pTransport(ri.host, 2*time.Second); err != nil {
		return err
	}
	ri.consensus = p2praft.NewConsensus(&MarshableState{State: &pb.State{Jobs: nil}})

	snapshots, err := raft.NewFileSnapshotStore(ri.filestorePath, 3, nil)
	if err != nil {
		return err
	}
	// InmemStore implements the LogStore and StableStore interface. It should NOT EVER be used for production. It is used only for unit tests. TODO Use the MDBStore implementation instead.
	logStore := raft.NewInmemStore()

  // For defaults, see https://github.com/hashicorp/raft/blob/ace424ea865b24a1ea66087d375051687b9fd404/config.go#L295
  // cfg.ShutdownOnRemove = false
	cfg := raft.DefaultConfig() 
	cfg.LocalID = raft.ServerID(ri.AddrInfo().GetId())
  cfg.LogOutput = ri.logger.Writer()

	bootstrapped, err := raft.HasExistingState(logStore, logStore, snapshots)
	if err != nil {
		return err
	} else if !bootstrapped {
    if err := raft.BootstrapCluster(cfg, logStore, logStore, snapshots, ri.transport, raft.Configuration{Servers: ri.servers}); err != nil {
      return err
    }
	}

	if ri.raft, err = raft.NewRaft(cfg, ri.consensus.FSM(), logStore, logStore, snapshots, ri.transport); err != nil {
		return err
	}
	ri.actor = p2praft.NewActor(ri.raft)
	ri.consensus.SetActor(ri.actor)
	go ri.observeLeadership(context.Background())
  return nil
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
      ri.logger.Println("observeLeadership context timed out; ending listening loop")
			return
		}
		if l := ri.Leader(); l != last {
      select {
      case ri.newLeader <- struct{}{}:
			  last = l
      default:
        ri.logger.Println("observeLeadership: chan newLeader is full")
      }
		}
	}
}

func (ri *RaftImpl) Leader() string {
  if ri.raft != nil && ri.raft.State() == raft.Leader {
    return ri.AddrInfo().GetId()
  }
  _, id := ri.raft.LeaderWithID()
  return (string)(id)
}

func (ri *RaftImpl) Commit(s *pb.State) (*pb.State, error) {
	if ri.raft.State() == raft.Leader {
    agreedState, err := ri.consensus.CommitState(&MarshableState{State: s}) // Blocking
		if err != nil {
		return nil, err
		}
		if agreedState == nil {
			return nil, fmt.Errorf("agreedState is nil: commited on a non-leader?")
		}
	}
  // Commit() is ignored when not leader
  return ri.Get()
}

func (ri *RaftImpl) bootstrapState() (*pb.State, error) {
  s := &pb.State{
    Jobs: make(map[string]*pb.Job),
  }
  return ri.Commit(s)
}

func (ri *RaftImpl) Get() (*pb.State, error) {
  v, err := ri.getState()
  if err != nil {
    if err == p2praft.ErrNoState {
      v, err = ri.bootstrapState()
      if err != nil {
        return nil, fmt.Errorf("bootstrap error: %w", err)
      }
    } else {
      return nil, fmt.Errorf("state.Get() error: %w (did you bootstrap?)\n", err)
    }
  }
  return v, nil
}

func (ri *RaftImpl) getState() (*pb.State, error) {
	s, err := ri.consensus.GetCurrentState()
	if err != nil {
		return nil, err
	}
	rs, ok := s.(*MarshableState)
	if !ok {
		return nil, fmt.Errorf("State type assertion failed")
	}
  if rs.State == nil {
    return nil, fmt.Errorf("nil raft state")
  }
  if rs.State.Jobs == nil {
    // For some reason we sometimes deserialize "empty map" as a nil map.
    rs.State.Jobs = make(map[string]*pb.Job)
  }
	return rs.State, nil
}
