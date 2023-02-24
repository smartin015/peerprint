package registry

import (
  "testing"
  "sync"
  "time"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "errors"
  "log"
  "path/filepath"
  "context"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
)

func testLocalRegistry(t *testing.T) *Registry {
  dir := t.TempDir()
  r, err := New(context.Background(), filepath.Join(dir, "registry.sqlite3"), true, pplog.New("registry", log.Default()))
  if err != nil {
    panic(err)
  }
  return r
}

func getReg(r *Registry, tbl string) ([]*pb.Network) {
  nn := []*pb.Network{}
  cur := make(chan *pb.Network)
  var wg sync.WaitGroup
  wg.Add(1)
  go func() {
    defer wg.Done()
    for n := range cur {
      nn = append(nn, n)
    }
  }()
  if err := r.GetRegistry(context.Background(), cur, tbl, true); err != nil {
    panic(err)
  }
  wg.Wait()
  return nn
}


func TestCountSyncErr(t *testing.T) {
  r := testLocalRegistry(t)
  r.countSyncErr(errors.New("failed to dial"))
  if r.Counters.DialFailure != 1 {
    t.Errorf("Expected dial failure=1, got %d", r.Counters.DialFailure)
  }

  r.countSyncErr(errors.New("ducks are pretty great"))
  if r.Counters.Other != 1 {
    t.Errorf("Expected other failure=1, got %d", r.Counters.Other)
  }
}

func TestTwoPeersOneNetwork(t *testing.T) {
  r1 := testLocalRegistry(t)
  r2 := testLocalRegistry(t)
  ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
  defer cancel()

  if err := r2.UpsertConfig(&pb.NetworkConfig{
    Creator: r2.t.ID(),
  }, []byte(""), RegistryTable); err != nil {
    panic(err)
  }

  go r1.Run(ctx)
  go r2.Run(ctx)

  r1.Await(ctx)
  r2.Await(ctx)
  if got := getReg(r1, LobbyTable); len(got) != 1 || got[0].Config.Creator != r2.t.ID() {
    t.Errorf("r1 lobby: want ID %v, got %v", r2.t.ID(), got)
  }
}
