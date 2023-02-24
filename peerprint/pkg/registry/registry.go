package registry

import (
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  "database/sql"
  "sync"
  "strings"
  "context"
  "fmt"
)

const (
	Rendezvous = "peerprint-registry-0.0.1"
)

type Counters struct {
  DialFailure int64
  BadProtocol int64
  Canceled int64
  InboundExceeded int64
  OutboundExceeded int64
  Ok int64
  Other int64
}

type Registry struct {
  path string
  db *sql.DB
  l *pplog.Sublog
  t transport.Interface
  Counters Counters
  mut sync.Mutex
  notifyCh chan struct{}
}

func New(ctx context.Context, dbPath string, local bool, l *pplog.Sublog) (*Registry, error) {
  kpriv, kpub, err := crypto.GenKeyPair()
  if err != nil {
    return nil, fmt.Errorf("keygen: %w", err)
  }
  r := &Registry {
    l: l,
    notifyCh: make(chan struct{}, 2),
  }
  if err := r.initStorage(dbPath); err != nil {
    return nil, fmt.Errorf("registry db init: %w", err)
  }

  t, err := transport.New(&transport.Opts{
    Addr: "/ip4/0.0.0.0/tcp/0",
    OnlyRPC: true,
    Rendezvous: Rendezvous,
    Local: local,
    PrivKey: kpriv,
    PubKey: kpub,
    PSK: nil,
  }, ctx, pplog.New("transport", r.l))
  if err != nil {
    return nil, fmt.Errorf("transport init: %w", err)
  }
  r.t = t

  return r, nil
}

func (r *Registry) Run(ctx context.Context) error {
  if !r.mut.TryLock() {
    return fmt.Errorf("Already running")
  }
  defer r.mut.Unlock()
  defer r.t.Destroy()
  s := NewServer(r.t, r, pplog.New("regsrv", r.l))

  if err := r.t.Run(ctx); err != nil {
    return fmt.Errorf("Error running transport: %w", err)
  }
  r.l.Info("Running discovery loop")
  for {
    select {
    case p := <-r.t.PeerDiscovered():
      n, err := s.SyncPeer(ctx, p.ID)
      r.l.Info("SyncPeer(%s): %d, %v", p.ID.Pretty(), n, err)
      r.countSyncErr(err)
      r.notify()
    case <-ctx.Done():
      r.l.Info("Context cancelled; exiting discovery loop")
      return nil
    }
  }
}

func (r *Registry) countSyncErr(err error) {
  if err == nil {
    r.Counters.Ok += 1
    return
  }
  estr := err.Error()
  if strings.Contains(estr, "failed to dial") {
    r.Counters.DialFailure += 1
  } else if strings.Contains(estr, "protocol not supported") {
    r.Counters.BadProtocol += 1
  } else if strings.Contains(estr, "canceled with error code") {
    r.Counters.Canceled += 1
  } else if strings.Contains(estr, "cannot reserve inbound connection") {
    r.Counters.InboundExceeded += 1
  } else if strings.Contains(estr, "cannot reserve outbound connection") {
    r.Counters.OutboundExceeded += 1
  } else {
    r.l.Error("SyncPeer() abnormal error: %+v", err)
    r.Counters.Other += 1
  }
}

func (r *Registry) notify() {
  select {
    case r.notifyCh <- struct{}{}:
    default:
  }
}

func (r *Registry) Await(ctx context.Context) {
  select {
  case <-r.notifyCh:
  case <-ctx.Done():
  }
}
