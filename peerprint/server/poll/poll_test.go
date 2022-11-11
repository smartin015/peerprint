package poll

import (
  "time"
  "testing"
  "context"
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
	//"google.golang.org/protobuf/proto"
)

const (
  minWait = 1*time.Millisecond
)

func testEnv(t *testing.T) *PollerImpl {
  ctx, cancel := context.WithCancel(context.Background())
  t.Cleanup(cancel)
  return New(ctx)
}

func testEnvPolling(t *testing.T) *PollerImpl {
  p := testEnv(t)
  p.SetPollPeriod(minWait)
  p.Resume()
  <-p.Epoch()
  p.Pause()
  return p
}

func TestEpoch(t *testing.T) {
  p := testEnv(t)
  p.SetPollPeriod(minWait)

  p.Resume()
  got := <-p.Epoch()
  if got < 0 || got > 1.0 {
    t.Errorf("Want value between [0, 1.0], got %v", got) 
  }
}

func TestUpdateNotPolling(t *testing.T) {
  p := testEnv(t)
  p.Update(&pb.PeerStatus{Id: "test"})
  if got := len(p.pollResult); got != 0 {
    t.Errorf("len(p.pollResult): want %v got %v", 0, got)
  }
}

func TestUpdate(t *testing.T) {
  p := testEnvPolling(t)
  want := &pb.PeerStatus{Id: "test"}
  p.Update(want)
  //if got := p.pollResult[0]; !proto.Equal(got, want) {
  //  t.Errorf("Update() not applied - want %v got %v", want, got)
  //}
}

func TestUpdateAtMaxEndsPoll(t *testing.T) {
  p := testEnv(t)
  p.maxPoll = 1
  p.SetPollTimeout(time.Hour) // Ensure test times out if not cut short
  p.SetPollPeriod(minWait)

  peer := &pb.PeerStatus{Id: "test"}

  p.Resume()
  <-p.Epoch()
  p.Pause()

  p.Update(peer)
  <-p.Result() // Simply returning from this means it worked
}

func TestStatsAtMax(t *testing.T) {
  p := testEnv(t)
  p.maxPoll = 5
  p.SetPollPeriod(minWait)

  peer := &pb.PeerStatus{Id: "test"}
  p.Resume()
  prob := <-p.Epoch()
  p.Pause()

  for i := 0; i < p.maxPoll; i++ {
    p.Update(peer)
  }
  summary := <-p.Result()
  want := int64(5/prob)
  if got := summary.PeerEstimate; got != want {
    t.Errorf("Summary got %v, want %v", got, want)
  }
}

