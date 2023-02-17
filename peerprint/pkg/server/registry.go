package server

import (
  "sync"
  "context"
  "fmt"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/libp2p/go-libp2p/core/peer"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
)

const (
  MaxConcurrentSync = 1
)

type Registry interface {
  SyncPeer(context.Context, peer.ID) (int, error)
}

type registry struct {
  t transport.Interface
  st storage.Registry
  l *pplog.Sublog
}

type RegistryService struct {
  base *registry
}

func (s *RegistryService) GetNetworks(ctx context.Context, reqChan <-chan struct{}, repChan chan<- *pb.Network) error {
  s.base.signNew(ctx)
  return s.base.st.GetRegistry(ctx, repChan, storage.RegistryTable, true) // repChan closed by impl
}

func NewRegistry(t transport.Interface, st storage.Registry, l *pplog.Sublog) Registry {
  srv := &registry {
    t: t,
    st: st,
    l: l,
  }
  if err := t.Register(PeerPrintProtocol, srv.getService()); err != nil {
    panic(fmt.Errorf("Failed to register RPC server: %w", err))
  }
  return srv
}

func (s *registry) getService() *RegistryService {
  return &RegistryService{
    base: s,
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

  if err := s.st.GetRegistry(ctx, nChan, storage.RegistryTable, true); err != nil {
    s.l.Error(err)
    return
  }
  wg.Wait()
  s.l.Info("Signed %d new records", nsigned)
}

func (s *registry) SyncPeer(ctx context.Context, p peer.ID) (int, error) {
  req := make(chan struct{}); close(req)
  rep := make(chan *pb.Network, 5) // Closed by Stream
  n := 0

  var wg sync.WaitGroup
  var streamErr error
  wg.Add(1)
  go func () {
    defer wg.Done()
    if p.String() == s.ID() {
      streamErr = s.getService().GetNetworks(ctx, req, rep)
    } else {
      streamErr = s.t.Stream(ctx, p, "GetNetworks", req, rep) 
    }
  }()
  for {
    select {
    case v, ok := <-rep:
      if !ok {
        wg.Wait()
        return n, streamErr
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
      s.l.Error("SyncPeer() context cancelled")
      return n, streamErr
    }
  }
}
