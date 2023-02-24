package registry

import (
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
	libp2p_crypto "github.com/libp2p/go-libp2p/core/crypto"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "time"
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
  Deadline time.Time
}

type Registry struct {
  path string
  db *sql.DB
  kpriv libp2p_crypto.PrivKey
  kpub libp2p_crypto.PubKey
  local bool
  l *pplog.Sublog
  Counters Counters
  mut sync.Mutex
}

func New(ctx context.Context, dbPath string, local bool, l *pplog.Sublog) (*Registry, error) {
  if err := initStorage(dbPath); err != nil {
    return nil, fmt.Errorf("registry db init: %w", err)
  }
  kpriv, kpub, err := crypto.GenKeyPair()
  if err != nil {
    return nil, fmt.Errorf("keygen: %w", err)
  }
  r := &Registry {
    DB: st,
    kpriv: kpriv,
    kpub: kpub,
    local: local,
    l: l,
  }
  return r, nil
}

func (r *Registry) Run(d time.Duration) error {
  if !r.mut.TryLock() {
    return fmt.Errorf("Already running")
  }
  defer r.mut.Unlock()
  r.Counters.Deadline = time.Now().Add(d)
  ctx, cancel := context.WithTimeout(context.Background(), d)
  defer cancel()
  t, err := transport.New(&transport.Opts{
    Addr: "/ip4/0.0.0.0/tcp/0",
    OnlyRPC: true,
    Rendezvous: Rendezvous,
    Local: r.local,
    PrivKey: r.kpriv,
    PubKey: r.kpub,
    PSK: nil,
  }, ctx, pplog.New("transport", r.l))
  defer t.Destroy()
  if err != nil {
    return fmt.Errorf("transport init: %w", err)
  }
  s := server.NewRegistry(t, r.DB, pplog.New("regsrv", r.l))

  if err := t.Run(ctx); err != nil {
    return fmt.Errorf("Error running transport: %w", err)
  }
  r.l.Info("Running discovery loop")
  for {
    select {
    case p := <-t.PeerDiscovered():
      n, err := s.SyncPeer(ctx, p.ID)
      r.l.Info("SyncPeer(%s): %d, %v", p.ID.Pretty(), n, err)
      if err == nil {
        r.Counters.Ok += 1
        return nil
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
      }
    case <-ctx.Done():
      r.l.Info("Context cancelled; exiting discovery loop")
      return nil
    }
  }
}
