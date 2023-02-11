package registry

import (
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "context"
  "fmt"
  "time"
)

const (
  DefaultSyncPd = 60*5*time.Second
	Rendezvous = "peerprint-registry-0.0.1"
)

type Registry struct {
  DB storage.Registry
  Srv server.Registry
}

func New(ctx context.Context, dbPath string, local bool, l *pplog.Sublog) (*Registry, error) {
  st, err := storage.NewRegistry(dbPath)
  if err != nil {
    return nil, fmt.Errorf("registry db init: %w", err)
  }
  kpriv, kpub, err := crypto.GenKeyPair()
  if err != nil {
    return nil, fmt.Errorf("keygen: %w", err)
  }
  t, err := transport.New(&transport.Opts{
    Addr: "/ip4/0.0.0.0/tcp/0",
    OnlyRPC: true,
    Rendezvous: Rendezvous,
    Local: local,
    PrivKey: kpriv,
    PubKey: kpub,
    PSK: nil,
  }, ctx, pplog.New("transport", l))
  if err != nil {
    return nil, fmt.Errorf("transport init: %w", err)
  }
  s := server.NewRegistry(t, st, DefaultSyncPd,
    pplog.New("regserver", pplog.New("regsrv", l)))
  go s.Run(ctx)

  return &Registry {
    Srv: s,
    DB: st,
  }, nil
}

