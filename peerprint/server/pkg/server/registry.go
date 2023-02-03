package server

import (
  "sync"
  "context"
  "time"
  "fmt"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/libp2p/go-libp2p/core/peer"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
)


type Registry interface {
  Run(context.Context)
}

type registry struct {
  t transport.Interface
  st storage.Registry
  syncTicker *time.Ticker
  l *pplog.Sublog
}

type RegistryService struct {
  base *registry
}

func (s *RegistryService) GetNetworks(ctx context.Context, reqChan <-chan struct{}, repChan chan<- *pb.Network) error {
  s.base.signNew(ctx)
  return s.base.st.GetNetworks(ctx, repChan) // repChan closed by impl
}

func NewRegistry(t transport.Interface, st storage.Registry, syncPeriod time.Duration, l *pplog.Sublog) Registry {
  srv := &registry {
    t: t,
    st: st,
    syncTicker: time.NewTicker(syncPeriod),
    l: l,
  }
  if err := t.Register(PeerPrintProtocol, srv.getService()); err != nil {
    panic(fmt.Errorf("Failed to register RPC server: %w", err))
  }
  return srv
}

func (s *registry) getService() interface{} {
  return &RegistryService{
    base: s,
  }
}

func (s *registry) Run(ctx context.Context) {
  s.t.Run(ctx)
  s.l.Info("Completing initial sync")
  time.Sleep(2*time.Second)
  s.sync(ctx)
  s.l.Info("Running server main loop")
  for {
    select {
    case <-s.syncTicker.C:
      go s.sync(ctx)
    case <-ctx.Done():
      s.l.Info("Context cancelled; exiting registry loop")
      return
    }
  }
}

func (s *registry) ID() string {
  return s.t.ID()
}

func (s *registry) signNew(ctx context.Context) {
  // Sign any newly added configs
  nChan := make(chan *pb.Network, 5)
  var wg sync.WaitGroup
  wg.Add(1)
  nsigned := 0
  go func () {
    defer wg.Done()
    for n := range nChan {
      if n.Config.Creator == "" && len(n.Signature) != 0 {
        continue
      }

      n.Config.Creator = s.ID()
      sig, err := s.t.Sign(n.Config)
      if err != nil {
        s.l.Error("Sign(%v): %w", n.Config, err)
        continue
      }
      if err := s.st.UpsertConfig(n.Config, sig, storage.RegistryTable); err != nil {
        s.l.Error("UpsertConfig(%v, %s): %w", n, sig, err)
        continue
      }
      nsigned++
    }
  }()

  if err := s.st.GetNetworks(ctx, nChan); err != nil {
    s.l.Error(err)
    return
  }
  wg.Wait()
  s.l.Info("Signed %d new records", nsigned)
}

func (s *registry) sync(ctx context.Context) {
  var wg sync.WaitGroup
  for _, p := range s.t.GetPeers() {
    if p.String() == s.ID() {
      continue // No self connection
    }
    wg.Add(1)
    s.l.Info("Syncing lobby from %s", shorten(p.String()))
    go func(p peer.ID) {
      defer storage.HandlePanic()
      defer wg.Done()
      n := s.syncPeer(ctx, p)
      s.l.Info("Synced %d rows from %s", n, shorten(p.String()))
    }(p)
  }
  wg.Wait()
}

func (s *registry) syncPeer(ctx context.Context, p peer.ID) int {
  req := make(chan struct{}); close(req)
  rep := make(chan *pb.Network, 5) // Closed by Stream
  n := 0
  go func () {
    defer storage.HandlePanic()
    if err := s.t.Stream(ctx, p, "GetNetworks", req, rep); err != nil {
      s.l.Error("syncPeer(): %+v", err)
      return
    }
  }()
  for {
    select {
    case v, ok := <-rep:
      if !ok {
        return n
      }
      // Ensure signature is valid
      if ok, err := s.t.Verify(v.Config, v.Config.Creator, v.Signature); err != nil {
        s.l.Warning("verify(): %v", err)
        continue
      } else if !ok {
        s.l.Warning("ignored (invalid signature) %s", v.Config.Uuid)
        continue
      }
      if err := s.st.UpsertConfig(v.Config, v.Signature, storage.LobbyTable); err != nil {
        s.l.Error("UpsertConfig(%v): %v", v, err)
      } else if err := s.st.UpsertStats(v.Config.Uuid, v.Stats); err != nil {
        s.l.Error("UpsertStats(%v): %v", v, err)
      } else {
        n += 1
      }
    case <-ctx.Done():
      s.l.Error("syncPeer() context cancelled")
      return n
    }
  }
}
