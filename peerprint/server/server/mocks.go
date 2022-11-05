package server

import (
  "testing"
  "time"
  "github.com/libp2p/go-libp2p"
  "github.com/libp2p/go-libp2p/core/host"
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
  "google.golang.org/protobuf/proto"
	tr "github.com/smartin015/peerprint/peerprint_server/topic_receiver"
  "context"
  "log"
)

const (
  TestID = "self"
  TestRAFTId = "raftid"
  TestTopic = "testtopic"
  TrustedPeer = "trustedpeer"
)

type TestEnv struct {
  CmdRecv chan proto.Message
  CmdSend chan proto.Message
  CmdPush chan proto.Message
  Sub chan tr.TopicMsg
  Opened map[string](chan proto.Message)
  Err chan error
  t *testing.T
  P *Server
  R *testRaft
  L *testPoller
}
func (e *TestEnv) Open(topic string) (chan<- proto.Message, error) {
  e.Opened[topic] = make(chan proto.Message, 5)
  return e.Opened[topic], nil
}
func testEnv(t *testing.T, trusted bool, asLeader bool) *TestEnv {
  ctx, cancel := context.WithCancel(context.Background())
  t.Cleanup(cancel)
  e := &TestEnv{
    CmdRecv: make(chan proto.Message, 5),
    CmdSend: make(chan proto.Message, 5),
    CmdPush: make(chan proto.Message, 5),
    Opened: make(map[string](chan proto.Message)),
    Sub: make(chan tr.TopicMsg),
    Err: make(chan error),
    t: t,
  }
  e.R = &testRaft{
    id: TestRAFTId,
    s: &pb.State{Jobs: make(map[string]*pb.Job)},
    lc: make(chan struct{}),
  }
  if asLeader {
    e.R.l = TestID
  }
  trust := []string{TrustedPeer}
  if trusted {
    trust = append(trust, TestID)
  }

  e.L = &testPoller{
    epoch: make(chan float64),
    result: make(chan *pb.PeersSummary),
  }
  t.Cleanup(func() {close(e.L.epoch); close(e.L.result)})
  e.P = New(ServerOptions{
    ID: TestID,
    TrustedPeers: trust,
    Logger: log.Default(),
    Raft: e.R,
    Poller: e.L,

    RecvPubsub: e.Sub,
    RecvCmd: e.CmdRecv,
    SendCmd: e.CmdSend,
    PushCmd: e.CmdPush,

    Opener: e.Open,
  })
  go e.P.Loop(ctx)
  return e
}

func withAssignment(pp *TestEnv) *TestEnv {
  // Pop off assignment request
  <-pp.Opened[AssignmentTopic]

  if _, err := pp.P.Handle(AssignmentTopic, TrustedPeer, &pb.AssignmentResponse{
    LeaderId: pp.P.getID(),
    Id: pp.P.getID(),
    Topic: TestTopic,
    Type: pb.PeerType_LISTENER,
  }); err != nil {
    pp.t.Fatalf(err.Error())
  }
  pp.P.l.Println("Assigned")
  return pp
}

func testLock(peer string) *pb.Lock {
  return &pb.Lock{Peer: peer, Created: uint64(time.Now().Unix())}
}

func testHost(t *testing.T) host.Host {
  t.Helper()
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
  if err != nil {
    t.Fatal(err)
  }
	t.Cleanup(func() {
		h.Close()
    h.ConnManager().Close()
	})
  return h
}

type testRaft struct {
  id string
  s *pb.State
  l string
  lset string
  lc chan struct{}
  peers []*pb.AddrInfo
}
func (ri *testRaft) ID() string { return ri.id }
func (ri *testRaft) Addrs() []string { return []string{}}
func (ri *testRaft) SetPeers(peers []*pb.AddrInfo) error {
  ri.peers = peers
  return nil
}
func (ri *testRaft) GetPeers() []*pb.AddrInfo {
  return ri.peers
}
func (ri *testRaft) Leader() string {
  return ri.l
}
func (ri *testRaft) LeaderChan() (chan struct{}) {
  return ri.lc
}
func (ri *testRaft) Get() (*pb.State, error) {
  return ri.s, nil
}
func (ri *testRaft) Commit(s *pb.State) (*pb.State, error) {
  ri.s = s
  return s, nil
}
func (ri *testRaft) SetLeader(l string) {
  ri.lset = l // Don't actually change leadership, as unit tests
  // sometimes init with the leader set to the server under test
  // which is needed for e.g. job mutations to be handled
}

type testPoller struct {
  epoch chan float64
  responses []*pb.PeerStatus
  result chan *pb.PeersSummary
}
func (p *testPoller)  Pause() {}
func (p *testPoller)  Resume() {}
func (p *testPoller)  Update(status *pb.PeerStatus) {
  p.responses = append(p.responses, status)
}
func (p *testPoller)  Epoch() (<-chan float64) {return p.epoch}
func (p *testPoller)  Result() (<-chan *pb.PeersSummary) {return p.result}

