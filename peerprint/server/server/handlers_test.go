package server
import (
  "time"
  "testing"
  "strings"
  "context"
  "reflect"
  "log"
  "github.com/libp2p/go-libp2p"
  "google.golang.org/protobuf/proto"
  "github.com/libp2p/go-libp2p-core/host"
  "google.golang.org/protobuf/types/known/wrapperspb"
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
	tr "github.com/smartin015/peerprint/peerprint_server/topic_receiver"
)

const (
  TestTopic = "testtopic"
  TrustedPeer = "trustedpeer"
  TestID = "self"
  TestRAFTId = "raftid"
)

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
  s *pb.State
  l string
  lset string
  lc chan struct{}
  peers []*pb.AddrInfo
}
func (ri *testRaft) ID() string { return TestRAFTId }
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
  result chan *pb.PeersSummary
}
func (p *testPoller)  Pause() {}
func (p *testPoller)  Resume() {}
func (p *testPoller)  Update(status *pb.PeerStatus) {}
func (p *testPoller)  Epoch() (<-chan float64) {return p.epoch}
func (p *testPoller)  Result() (<-chan *pb.PeersSummary) {return p.result}


type TestEnv struct {
  CmdRecv chan proto.Message
  CmdSend chan proto.Message
  CmdPush chan proto.Message
  Sub chan tr.TopicMsg
  Opened map[string](chan proto.Message)
  Err chan error
  P *Server
  R *testRaft
}
func (e *TestEnv) Open(topic string) (chan proto.Message, error) {
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
  }
  e.R = &testRaft{
    s: &pb.State{Jobs: make(map[string]*pb.Job)},
  }
  if asLeader {
    e.R.l = TestID
  }
  trust := []string{TrustedPeer}
  if trusted {
    trust = append(trust, TestID)
  }

  tp := &testPoller{
    epoch: make(chan float64),
    result: make(chan *pb.PeersSummary),
  }
  t.Cleanup(func() {close(tp.epoch); close(tp.result)})
  e.P = New(ServerOptions{
    ID: TestID,
    TrustedPeers: trust,
    Logger: log.Default(),
    Raft: e.R,
    Poller: tp,

    RecvPubsub: e.Sub,
    RecvCmd: e.CmdRecv,
    SendCmd: e.CmdSend,
    PushCmd: e.CmdPush,

    Opener: e.Open,
  })
  go e.P.Loop(ctx)
  return e
}

func TestAssignmentRequest(t *testing.T) {
  pp := testEnv(t, true, true)
  rep, err := pp.P.OnAssignmentRequest(AssignmentTopic, "asdf", &pb.AssignmentRequest{})
  if err != nil {
    t.Errorf("handle assignment request: got err = %v", err)
  } else if rep.Id != "asdf" || rep.Topic != DefaultTopic || rep.Type == pb.PeerType_UNKNOWN_PEER_TYPE {
    t.Errorf("handle assignment request got invalid reply: Id, topic, or type incorrect: %v", rep)
  }
}

func TestPollPeersRequest(t *testing.T) {
  t.Skip("TODO")
  pp := testEnv(t, true, true)
  rep, err := pp.P.OnPollPeersRequest(TestTopic, "asdf", &pb.PollPeersRequest{Probability: 0.0})
  if err != nil {
    t.Errorf("poll request: got err = %v", err)
  } else if rep != nil {
    t.Errorf("poll request: got reply when guaranteed silence")
  }
  rep, err = pp.P.OnPollPeersRequest(TestTopic, "asdf", &pb.PollPeersRequest{Probability: 1.0})
  if err != nil {
    t.Errorf("poll request: got err = %v", err)
  } else if rep == nil {
    t.Errorf("poll request: nil reply when guaranteed response")
  } else if rep.Status.Id != "leader" {
    t.Errorf("poll request: response id mismatch - want leader got %q", rep.Status.Id)
  }
}

func TestRaftAddrsRequest(t *testing.T) {
  pp := testEnv(t, true, true)
  h := testHost(t)
  req := &pb.RaftAddrsRequest{
    AddrInfo: &pb.AddrInfo{
      Id: h.ID().Pretty(),
      Addrs: []string{h.Addrs()[0].String()},
    },
  }
  rep, err := pp.P.OnRaftAddrsRequest(DefaultTopic, TrustedPeer, req)
  if err != nil {
    t.Errorf("raft request: got err = %v", err)
  } else if rep == nil {
    t.Errorf("raft request: nil reply, wanted reply")
  } else if want:= pp.P.raftAddrsResponse(); !proto.Equal(rep, want) {
    t.Errorf("raft request: got %v want %v", rep, want)
  }
}

func TestAssignmentResponseAsListener(t *testing.T) {
  pp := testEnv(t, false, false)

  req := <-pp.Opened[AssignmentTopic]
  if req == nil {
    t.Errorf("want assignment request, got nil")
  }

  wantLeader := TrustedPeer
  resp, err := pp.P.Handle(AssignmentTopic, TrustedPeer, &pb.AssignmentResponse{
    LeaderId: wantLeader,
    Id: pp.P.getID(),
    Topic: TestTopic,
    Type: pb.PeerType_LISTENER,
  })
  if (err != nil) || (resp != (*pb.RaftAddrsRequest)(nil)) {
    t.Errorf("want nil, nil, got (%T, %v), (%T, %v)", resp, resp, err, err)
  } else if got := pp.P.getTopic(); got != TestTopic {
    t.Errorf("new topic not assigned: want %q got %q", TestTopic, got)
  } else if got := pp.R.lset; got != wantLeader {
    t.Errorf("new leader not assigned: want %q got %q", wantLeader, got)
  }
}

func TestAssignmentResponseAsElectable(t *testing.T) {
  pp := testEnv(t, false, false)

  req := <-pp.Opened[AssignmentTopic]
  if req == nil {
    t.Errorf("want assignment request, got nil")
  }

  wantLeader := TrustedPeer
  resp, err := pp.P.Handle(AssignmentTopic, TrustedPeer, &pb.AssignmentResponse{
    LeaderId: wantLeader,
    Id: pp.P.getID(),
    Topic: TestTopic,
    Type: pb.PeerType_ELECTABLE,
  })
  if err != nil {
    t.Errorf("Want err=nil, got %v", err)
  }
  switch v := resp.(type) {
  case *pb.RaftAddrsRequest:
    if got := v.GetAddrInfo().GetId(); got != TestRAFTId {
      t.Errorf("Expected raft id %q on request, got %q", TestRAFTId, got)
    }
  default:
    t.Errorf("Want RaftAddrsRequest, got something else")
  }
}

func TestAssignmentResponseNotOurs(t *testing.T) {
  pp := testEnv(t, false, false)
  resp, err := pp.P.Handle(AssignmentTopic, TrustedPeer, &pb.AssignmentResponse{
    Id: "notus",
    Topic: TestTopic,
    LeaderId: TrustedPeer,
    Type: pb.PeerType_LISTENER,
  })
  if err != nil || resp != nil {
    t.Errorf("want nil, nil, got %v, %v", resp, err)
  } else if l := pp.P.getLeader(); l == TrustedPeer {
    t.Errorf("AssignmentResponse getLeader() want unchanged, got %q", l)
  } else if typ := pp.P.getType(); typ == pb.PeerType_LISTENER {
    t.Errorf("AssignmentResponse getType() want unchanged, got %v", typ)
  }
}
func TestPollPeersResponse(t *testing.T) {
  t.Skip("TODO")
}
func TestPeersSummary(t *testing.T) {
  t.Skip("TODO")
}
func TestRaftAddrsResponse(t *testing.T) {
  pp := testEnv(t, true, true)
  want := []*pb.AddrInfo{
      &pb.AddrInfo{
        Id: "A",
        Addrs: []string{"B", "C"},
    },
  }
  // Use assignment topic to avoid having to issue assignment
  _, err := pp.P.Handle(AssignmentTopic, TrustedPeer, &pb.RaftAddrsResponse{
    Peers: want,
  })
  if err != nil {
    t.Errorf(err.Error())
  }
  if got := pp.R.peers; !reflect.DeepEqual(got, want) {
    t.Errorf("Want %v got %v", want, got)
  }
  // TODO send RaftAddrsResponse, check peers added with pp.P.getRaftPeers()
}
func TestSetJobRequest(t *testing.T) {
  pp := testEnv(t, true, true)
  want := &pb.State{Jobs: map[string]*pb.Job{"foo": &pb.Job{Id: "foo"}}}
  got, err := pp.P.OnSetJobRequest(TestTopic, TrustedPeer, &pb.SetJobRequest{
    Job: want.Jobs["foo"],
  })
  if err != nil {
    t.Errorf(err.Error())
  }
  if !proto.Equal(got, want) {
    t.Errorf("want %v got %v", want, got)
  }
}
func TestSetJobRequestOnSelfLockedJob(t *testing.T) {
  t.Skip("TODO")
}
func TestSetJobRequestOnPeerLockedJob(t *testing.T) {
  t.Skip("TODO")
}
func TestDeleteJobRequest(t *testing.T) {
  pp := testEnv(t, true, true)
  pp.R.s = &pb.State{Jobs: map[string]*pb.Job{"foo": &pb.Job{Id: "foo"}}}
  want := &pb.State{Jobs: map[string]*pb.Job{}}
  got, err := pp.P.OnDeleteJobRequest(TestTopic, TrustedPeer, &pb.DeleteJobRequest{
    Id: "foo",
  })
  if err != nil || !proto.Equal(got, want) {
    t.Errorf("want %v, nil got %v, %v", want, got, err)
  }
}
func TestDeleteJobRequestOnSelfLockedJob(t *testing.T) {
  // Even self locked jobs should not allow deletion
  t.Skip("TODO")
}

func TestAcquireJobRequest(t *testing.T) {
  pp := testEnv(t, true, true)
  pp.R.s = &pb.State{Jobs: map[string]*pb.Job{"foo": &pb.Job{Id: "foo"}}}
  got, err := pp.P.OnAcquireJobRequest(TestTopic, TrustedPeer, &pb.AcquireJobRequest{
    Id: "foo",
  })
  if err != nil {
    t.Errorf(err.Error())
  } else if got2 := got.Jobs["foo"].GetLock().GetPeer(); got2 != TrustedPeer {
    t.Errorf("Job lock peer: got %v want %v", got2, TrustedPeer)
  }
}
func TestAcquireJobRequestOnSelfLockedJob(t *testing.T) {
  t.Skip("TODO")
}
func TestAcquireJobRequestOnPeerLockedJob(t *testing.T) {
  t.Skip("TODO")
}

func TestReleaseJobRequest(t *testing.T) {
  pp := testEnv(t, true, true)
  pp.R.s = &pb.State{Jobs: map[string]*pb.Job{
    "foo": &pb.Job{
      Id: "foo", 
      Lock: &pb.Lock{Peer: TrustedPeer, Created: uint64(time.Now().Unix())},
    },
  }}
  got, err := pp.P.OnReleaseJobRequest(TestTopic, TrustedPeer, &pb.ReleaseJobRequest{
    Id: "foo",
  })
  if err != nil {
    t.Errorf(err.Error())
  } else if got2 := got.Jobs["foo"].GetLock(); got2 != nil {
    t.Errorf("Job lock peer: got %v want nil", got2)
  }
}
func TestReleaseJobRequestOnPeerLockedJob(t *testing.T) {
  t.Skip("TODO")
}
func TestReleaseJobRequestOnUnlockedJob(t *testing.T) {
  t.Skip("TODO")
}

func TestState(t *testing.T) {
  pp := testEnv(t, false, false)
  pp.R.l = TrustedPeer
  want := &pb.State{Jobs: map[string]*pb.Job{"foo": &pb.Job{Id: "foo"}}}
  _, err := pp.P.Handle(TestTopic, TrustedPeer, want)
  if err != nil {
    t.Errorf(err.Error())
  }

  got, err := pp.R.Get()
  if err != nil {
    t.Errorf(err.Error())
  } else if !proto.Equal(got, want) {
    t.Errorf("want %v, got %v", want, got)
  }

  got2 := <-pp.CmdPush
  if !proto.Equal(got2, want) {
    t.Errorf("want %v, got %v", want, got2)
  }
  // TODO assert presence on command push
}

func TestHandleUnknownProto(t *testing.T) {
  pp := testEnv(t, false, false)
  _, err := pp.P.Handle(TestTopic, TrustedPeer, &wrapperspb.StringValue{Value: "what is this even"})
  if err != nil {
    t.Errorf("Expected no error, got %v", err)
  }
}
func TestHandleNonLeaderMutationRequest(t *testing.T) {
  pp := testEnv(t, false, false)
  _, err := pp.P.Handle(TestTopic, TrustedPeer, &pb.SetJobRequest{})
  if err != nil {
    t.Errorf("Expected no error, got %v", err)
  }
}
func TestMissingJobRequest(t *testing.T) {
  pp := testEnv(t, true, true)
  pp.P.raft.Commit(&pb.State{Jobs: make(map[string]*pb.Job)})
  _, err := pp.P.Handle(AssignmentTopic, TrustedPeer, &pb.AcquireJobRequest{
    Id: "notarealjob",
  })
  if err == nil {
    t.Errorf("Expected err, got %v", err)
  } else if !strings.Contains(err.Error(), "not found") {
    t.Errorf("Expected 'not found' error, got %v", err)
  }
}

func TestElectionRequestToListener(t *testing.T) {
  t.Skip("TODO")
}
