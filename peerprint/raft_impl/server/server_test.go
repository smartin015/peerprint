package server

import (
  "testing"
  "strings"
	"google.golang.org/protobuf/proto"
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
	tr "github.com/smartin015/peerprint/peerprint_server/topic_receiver"
)

func TestOnPollEpoch(t *testing.T) {
  pp := testEnv(t, true, true)
  pp.L.epoch<- 0.5
  got := <-pp.Opened[DefaultTopic]
  want := &pb.PollPeersRequest{Probability: 0.5}
  if !proto.Equal(got, want) {
    t.Errorf("got %v want %v", got, want)
  }
}
func TestOnPollResult(t *testing.T) {
  pp := testEnv(t, true, true)
  want := &pb.PeersSummary{PeerEstimate: 5}
  pp.L.result<- want
  got := <-pp.Opened[DefaultTopic]
  if !proto.Equal(got, want) {
    t.Errorf("got %v want %v", got, want)
  }
}
func TestOnLeaderElected(t *testing.T) {
  pp := testEnv(t, true, true)
  pp.R.s = &pb.State{Jobs: map[string]*pb.Job{
    "foo": &pb.Job{Id: "foo"},
  }}
  pp.R.l = pp.P.getID()
  pp.R.lc<- struct{}{}
  wantLeader := &pb.Leader{Id: pp.P.getID()}
  gotLeader := <-pp.Opened[DefaultTopic]
  if !proto.Equal(gotLeader, wantLeader) {
    t.Errorf("got %v want %v", gotLeader, wantLeader)
  }
  wantState := pp.R.s
  gotPub := <-pp.Opened[DefaultTopic]
  if !proto.Equal(gotPub, wantState) {
    t.Errorf("got %v want %v", gotPub, wantState)
  }
  gotPush := <-pp.CmdPush
  if !proto.Equal(gotPush, wantState) {
    t.Errorf("got %v want %v", gotPush, wantState)
  }
}

func TestOnStateChanged(t *testing.T) {
  pp := testEnv(t, true, true)
  pp.R.s = &pb.State{Jobs: map[string]*pb.Job{
    "foo": &pb.Job{Id: "foo"},
  }}
  pp.R.sc<- struct{}{}
  gotPush := <-pp.CmdPush
  if !proto.Equal(gotPush, pp.R.s) {
    t.Errorf("got %v want %v", gotPush, pp.R.s)
  }
}

func TestOnCommandReceivedAsLeader(t *testing.T) {
  pp := testEnv(t, true, true)
  want := &pb.Job{Id: "foo"}
  pp.CmdRecv<- &pb.SetJobRequest{Job: want}
  got := <-pp.CmdSend
  if !proto.Equal(got, &pb.Ok{}) {
    t.Errorf("Want OK, got %v", got)
  }
  got = <-pp.CmdPush
  if !proto.Equal(got, &pb.State{
    Jobs: map[string]*pb.Job{"foo": want},
  }) {
    t.Errorf("want state with job %v, got %v", want, got)
  }
}
func TestOnCommandReceivedAsLeaderError(t *testing.T) {
  pp := testEnv(t, true, true)
  pp.CmdRecv<- &pb.AcquireJobRequest{Id: "notfound"}
  switch got := (<-pp.CmdSend).(type) {
  case *pb.Error:
    if !strings.Contains(got.GetStatus(), "not found") {
      t.Errorf("Want 'not found' error, got %v", got)
    }
  default:
    t.Fatalf("Type error")
  }
}
func TestOnCommandReceivedAsListener(t *testing.T) {
  pp := withAssignment(testEnv(t, false, false))
  req := &pb.AcquireJobRequest{Id: "foo"}
  pp.CmdRecv<- req
  got := <-pp.CmdSend
  // Ensure we get a response to appease REQ/REP zmq protocol
  if !proto.Equal(got, &pb.Ok{}) {
    t.Errorf("Want OK, got %v", got)
  }
  // Ensure command is forwarded on to the leader
  got = <-pp.Opened[TestTopic]
  if !proto.Equal(got, req) {
    t.Errorf("want %v got %v", req, got)
  }
}

func TestOnPubsubMessage(t *testing.T) {
  pp := testEnv(t, true, true)
  req := &pb.SetJobRequest{Job: &pb.Job{Id: "foo"}}
  pp.Sub<- tr.TopicMsg{
    Topic: pp.P.getTopic(),
    Peer: TrustedPeer,
    Msg: req,
  }
  // Reply is sent back to the topic
  got := <-pp.Opened[pp.P.getTopic()]
  want := &pb.State{
    Jobs: map[string]*pb.Job{
      "foo": req.GetJob(),
    },
  }
  if !proto.Equal(got, want) {
    t.Errorf("want %v got %v", want, got)
  }
}
