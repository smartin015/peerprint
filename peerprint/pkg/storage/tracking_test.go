package storage 

import (
  "testing"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
)

func TestEventAppendGet(t *testing.T) {
  t.Skip("TODO");
}

func TestPeerStatusSetGet(t *testing.T) {
  db := testingDB(t)
  if got, err := pstatGet(db); err != nil || len(got) > 0 {
    t.Errorf("GetPeerStatuses -> %v, %v want len()==0, nil", got, err)
    return
  }

  want := &pb.PeerStatus{
    Name: "testpeer",
    Clients: []*pb.ClientStatus{
      &pb.ClientStatus{Name:"p1"},
      &pb.ClientStatus{Name:"p2"},
      &pb.ClientStatus{Name:"p3"},
    },
  }
  if err := db.SetPeerStatus("sid1", want); err != nil {
    t.Errorf("SetPeerStatus(sid1): %v", err)
  }
  if err := db.SetPeerStatus("sid2", want); err != nil {
    t.Errorf("SetPeerStatus(sid2): %v", err)
  }

  if got, err := pstatGet(db); err != nil || len(got) != 2 {
    t.Errorf("Get: %v, %v, want len=2, nil", got, err)
  }
}

func TestLogPeerCrawl(t *testing.T) {
  t.Skip("TODO");
}

func TestPeerTracking(t *testing.T) {
  t.Skip("TODO TrackPeer");
}

func TestGetPeerTimeline(t *testing.T) {
  t.Skip("TODO");
}

