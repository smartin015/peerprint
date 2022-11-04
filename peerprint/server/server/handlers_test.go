package server
import (
  "testing"
  "strings"
  "reflect"
  "google.golang.org/protobuf/proto"
  "google.golang.org/protobuf/types/known/wrapperspb"
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
)

const (
  TestTopic = "testtopic"
  TrustedPeer = "trustedpeer"
  TestRAFTId = "raftid"
)


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
  } else if rep.Status.Id != pp.P.getLeader() {
    t.Errorf("poll request: response id mismatch - want %q got %q", pp.P.getLeader(), rep.Status.Id)
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
  pp := testEnv(t, true, true)
  err := pp.P.OnPollPeersResponse(TestTopic, TrustedPeer, &pb.PollPeersResponse{
    Status: &pb.PeerStatus{Id: TrustedPeer},
  })
  if err != nil {
    t.Errorf(err.Error())
  }
  if got := len(pp.L.responses); got != 1 {
    t.Errorf("len(poller responses) got %v want %v", got, 1)
  }
}
func TestPeersSummary(t *testing.T) {
  pp := testEnv(t, true, true)
  want := &pb.PeersSummary{
    PeerEstimate: 1337,
    Variance: 0.5,
  }
  pp.P.OnPeersSummary(TestTopic, TrustedPeer, want)
  got := <-pp.CmdPush
  if got != want {
    t.Errorf("want %v got %v", want, got)
  }
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
}

func testJobMutation(t *testing.T, req proto.Message, lockPeer string) (*pb.Job, error) {
  pp := testEnv(t, true, true)
  pp.R.s = &pb.State{Jobs: map[string]*pb.Job{
    "foo": &pb.Job{Id: "foo"},
  }}
  if lockPeer != "" {
    pp.R.s.Jobs["foo"].Lock = testLock(lockPeer)
  }
  _, err := pp.P.Handle(TestTopic, TrustedPeer, req)
  if err != nil {
    return nil, err
  }
  j, _ := pp.R.s.Jobs["foo"]
  return j, nil
}

func TestSetJobRequest(t *testing.T) {
  want := &pb.Job{Id: "foo"}
  got, err := testJobMutation(t,
    &pb.SetJobRequest{Job: want},
    "",
  )
  if err != nil || !proto.Equal(got, want) {
    t.Errorf("want %v, %v; got %v, %v", want, nil, got, err)
  }
}
func TestSetJobRequestOnLockedJob(t *testing.T) {
  got, err := testJobMutation(t,
    &pb.SetJobRequest{Job: &pb.Job{Id: "foo"}},
    TrustedPeer, // Request made by trusted peer on job locked by the same
  )
  want := &pb.Job{Id: "foo", Lock: testLock(TrustedPeer)}

  // No need to test for timing
  got.Lock.Created = 0; want.Lock.Created = 0

  if err != nil || !proto.Equal(got, want) {
    t.Errorf("want %v, %v; got %v, %v", want, nil, got, err)
  }
}
func TestSetJobRequestOnPeerLockedJob(t *testing.T) {
  _, err := testJobMutation(t,
    &pb.SetJobRequest{Job: &pb.Job{Id: "foo"}},
    "nacho lock",
  )
  if err == nil || !strings.Contains(err.Error(), "access denied") {
    t.Errorf("want _, 'access denied' err; got _, %v", err)
  }
}
func TestDeleteJobRequest(t *testing.T) {
  got, err := testJobMutation(t,
    &pb.DeleteJobRequest{
      Id: "foo",
    },
    "",
  )
  if err != nil || got != nil {
    t.Errorf("want nil, nil got %v, %v", got, err)
  }
}
func TestDeleteJobRequestOnLockedJob(t *testing.T) {
  got, err := testJobMutation(t,
    &pb.DeleteJobRequest{
      Id: "foo",
    },
    TrustedPeer, // Request made by trusted peer, locked by same
  )
  if err != nil || got != nil {
    t.Errorf("want nil, nil got %v, %v", got, err)
  }
}
func TestDeleteJobRequestOnPeerLockedJob(t *testing.T) {
  _, err := testJobMutation(t,
    &pb.DeleteJobRequest{
      Id: "foo",
    },
    "nacho peer",
  )
  if err == nil || !strings.Contains(err.Error(), "access denied") {
    t.Errorf("want _, 'access denied' err; got _, %v", err)
  }
}

func TestAcquireJobRequest(t *testing.T) {
  got, err := testJobMutation(t,
    &pb.AcquireJobRequest{Id: "foo"},
    "",
  )
  want := &pb.Job{Id: "foo", Lock: testLock(TrustedPeer)}
  // No need to test for timing
  got.Lock.Created = 0; want.Lock.Created = 0
  if err != nil || !proto.Equal(got, want) {
    t.Errorf("want %v, %v; got %v, %v", want, nil, got, err)
  }
}
func TestAcquireJobRequestOnSelfLockedJob(t *testing.T) {
  got, err := testJobMutation(t,
    &pb.AcquireJobRequest{Id: "foo"},
    TrustedPeer, // Request made by trusted peer on job locked by the same
  )
  want := &pb.Job{Id: "foo", Lock: testLock(TrustedPeer)}
  // No need to test for timing
  got.Lock.Created = 0; want.Lock.Created = 0
  if err != nil || !proto.Equal(got, want) {
    t.Errorf("want %v, %v; got %v, %v", want, nil, got, err)
  }
}
func TestAcquireJobRequestOnPeerLockedJob(t *testing.T) {
  _, err := testJobMutation(t,
    &pb.AcquireJobRequest{
      Id: "foo",
    },
    "nacho peer",
  )
  if err == nil || !strings.Contains(err.Error(), "access denied") {
    t.Errorf("want _, 'access denied' err; got _, %v", err)
  }
}


func TestReleaseJobRequest(t *testing.T) {
  got, err := testJobMutation(t,
    &pb.ReleaseJobRequest{Id: "foo"},
    TrustedPeer,
  )
  want := &pb.Job{Id: "foo", Lock: nil}
  if err != nil || !proto.Equal(got, want) {
    t.Errorf("want %v, %v; got %v, %v", want, nil, got, err)
  }
}
func TestReleaseJobRequestOnUnlockedJob(t *testing.T) {
  got, err := testJobMutation(t,
    &pb.ReleaseJobRequest{Id: "foo"},
    "",
  )
  want := &pb.Job{Id: "foo", Lock: nil}
  if err != nil || !proto.Equal(got, want) {
    t.Errorf("want %v, %v; got %v, %v", want, nil, got, err)
  }
}
func TestReleaseJobRequestOnPeerLockedJob(t *testing.T) {
  _, err := testJobMutation(t,
    &pb.ReleaseJobRequest{Id: "foo"},
    "nacho peer",
  )
  if err == nil || !strings.Contains(err.Error(), "access denied") {
    t.Errorf("want _, 'access denied' err; got _, %v", err)
  }
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
