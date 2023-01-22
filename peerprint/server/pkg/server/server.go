package server

import (
  "sync"
  "context"
  "time"
  "fmt"
  "google.golang.org/protobuf/proto"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
	"github.com/smartin015/peerprint/p2pgit/pkg/transport"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/smartin015/peerprint/p2pgit/pkg/storage"
	"github.com/smartin015/peerprint/p2pgit/pkg/log"
)

const (
  PeerPrintProtocol = "peerprint@0.0.1"
  DefaultTopic = "default"
  StatusTopic = "status"
)

type Opts struct {
  SyncPeriod time.Duration
  StatusPeriod time.Duration
  DisplayName string

  MaxRecordsPerPeer int64
  MaxCompletionsPerPeer int64
  MaxTrackedPeers int64
}

type Interface interface {
  ID() string
  ShortID() string
  GetService() interface{}

  IssueRecord(r *pb.Record, publish bool) (*pb.SignedRecord, error)
  IssueCompletion(g *pb.Completion, publish bool) (*pb.SignedCompletion, error)
  Sync(context.Context)

  Run(context.Context)
  OnUpdate() <-chan struct{}
}

type Server struct {
  opts Opts
  t transport.Interface
  s storage.Interface
  l *log.Sublog

  updateChan chan struct{}

  // Tickers for periodic network activity
  publishStatusTicker *time.Ticker
  syncTicker *time.Ticker
  status *pb.PeerStatus
}

func New(t transport.Interface, s storage.Interface, opts *Opts, l *log.Sublog) *Server {
  srv := &Server{
    t: t,
    s: s,
    l: l,
    updateChan: make(chan struct{}),
    publishStatusTicker: time.NewTicker(opts.StatusPeriod),
    syncTicker: time.NewTicker(opts.SyncPeriod),
    status: &pb.PeerStatus{
      Name: opts.DisplayName,
    },
  }
  if err := s.SetPubKey(t.ID(), t.PubKey()); err != nil {
    panic(fmt.Errorf("Failed to store our pubkey: %w", err))
  }
  if err := t.Register(PeerPrintProtocol, srv.GetService()); err != nil {
    panic(fmt.Errorf("Failed to register RPC server: %w", err))
  }
  return srv
}

func (s *Server) ID() string {
  return s.t.ID()
}


func (s *Server) OnUpdate() <-chan struct{} {
  return s.updateChan
}

func pretty(i interface{}) string {
  switch v := i.(type) {
  case *pb.SignedCompletion:
    return fmt.Sprintf("Completion{signer %s: %s did %s}", shorten(v.Signature.Signer), shorten(v.Completion.Completer), v.Completion.Uuid)
  case *pb.Completion:
    return fmt.Sprintf("Completion{%s did %s}", shorten(v.Completer), v.Uuid)
  case *pb.Record:
    return fmt.Sprintf("Record{%s->%s (%d tags)}", shorten(v.Uuid), shorten(v.Location), len(v.Tags))
  case *pb.SignedRecord:
    return fmt.Sprintf("Record{signer %s: %s->%s (%d tags)}", shorten(v.Signature.Signer), shorten(v.Record.Uuid), shorten(v.Record.Location), len(v.Record.Tags))
  default:
    return fmt.Sprintf("%+v", i)
  }
}

func shorten(s string) string {
  if len(s) < 5 {
    return s
  }
  return s[len(s)-4:]
}

func (s *Server) ShortID() string {
  return shorten(s.ID())
}

func (s *Server) IssueCompletion(g *pb.Completion, publish bool) (*pb.SignedCompletion, error) {
  sig, err := s.sign(g)
  if err != nil {
    return nil, fmt.Errorf("sign completion: %w", err)
  }
  sg := &pb.SignedCompletion{
    Completion: g,
    Signature: sig,
  }
  s.l.Info("issueCompletion(%s)", pretty(sg))
  if err := s.s.SetSignedCompletion(sg); err != nil {
    return nil, fmt.Errorf("store completion: %w", err)
  }
  if publish {
    if err := s.t.Publish(DefaultTopic, g); err != nil {
      return nil, fmt.Errorf("publish completion: %w", err)
    }
  }
  return sg, nil
}

func (s *Server) IssueRecord(r *pb.Record, publish bool) (*pb.SignedRecord, error) {
  s.l.Info("issueRecord(%s)", pretty(r))
  sig, err := s.sign(r)
  if err != nil {
    return nil, fmt.Errorf("sign record: %w", err)
  }
  sr := &pb.SignedRecord{
    Record: r,
    Signature: sig,
  }
  if err := s.s.SetSignedRecord(sr); err != nil {
    return nil, fmt.Errorf("write record: %w", err)
  }
  if publish {
    if err := s.t.Publish(DefaultTopic, r); err != nil {
      return nil, fmt.Errorf("publish record: %w", err)
    }
  }
  return sr, nil
}

func (s *Server) sendStatus() error {
  return s.t.Publish(StatusTopic, s.status)
}

func (s *Server) syncRecords(ctx context.Context, p peer.ID) int {
  req := make(chan struct{}); close(req)
  rep := make(chan *pb.SignedRecord, 5) // Closed by Stream
  n := 0
  go func () {
    if err := s.t.Stream(ctx, p, "GetSignedRecords", req, rep); err != nil {
      s.l.Error("syncRecords(): %+v", err)
      return
    }
  }()
  for {
    select {
    case v, ok := <-rep:
      if !ok {
        return n
      }
      if err := s.s.SetSignedRecord(v); err != nil {
        s.l.Error("syncRecords() SetSignedRecord(%v): %v", v, err)
      } else {
        n += 1
      }
    case <-ctx.Done():
      s.l.Error("syncRecords() context cancelled")
      return n
    }
  }
}

func (s *Server) syncCompletions(ctx context.Context, p peer.ID) int {
  req := make(chan struct{}); close(req)
  rep := make(chan *pb.SignedCompletion, 5) // Closed by Stream
  n := 0
  go func () {
    if err := s.t.Stream(ctx, p, "GetSignedCompletions", req, rep); err != nil {
      s.l.Error("syncCompletions(): %+v", err)
      return
    }
  }()
  for {
    select {
    case v, ok := <-rep:
      if !ok {
        return n
      }
      if err := s.s.SetSignedCompletion(v); err != nil {
        s.l.Error("syncCompletions() SetSignedCompletion(%v): %v", v, err)
      } else {
        n += 1
      }
    case <-ctx.Done():
      s.l.Error("Timeout while syncing")
      return n
    }
  }
}

func (s *Server) Sync(ctx context.Context) {
  var wg sync.WaitGroup
  for _, p := range s.t.GetPeers() {
    if p.String() == s.ID() {
      continue // No self connection
    }
    // TODO fetch <= MaxRecordsPerPeer records and MaxCompletionsPerPeer completions for p
    wg.Add(2)
    s.l.Info("Syncing with %s", shorten(p.String()))
    go func(p peer.ID) {
      defer wg.Done()
      n := s.syncRecords(ctx, p)
      s.l.Info("Synced %d records from %s", n, shorten(p.String()))
    }(p)
    go func(p peer.ID) {
      defer wg.Done()
      n := s.syncCompletions(ctx, p)
      s.l.Info("Synced %d completions from %s", n, shorten(p.String()))
    }(p)
  }
  wg.Wait()
}

func (s *Server) sign(m proto.Message) (*pb.Signature, error) {
  if b, err := proto.Marshal(m); err != nil {
    return nil, fmt.Errorf("sign() marshal error: %w", err)
  } else if sig, err := s.t.Sign(b); err != nil {
    return nil, fmt.Errorf("sign() crypto error: %w", err)
  } else {
    return &pb.Signature{
      Signer: s.t.ID(),
      Data: sig,
    }, nil
  }
}

func (s *Server) verify(m proto.Message, sig *pb.Signature) (bool, error) {
  k, err := s.s.GetPubKey(sig.Signer)
  if err != nil {
    return false, fmt.Errorf("verify() get key error: %w", err)
  }
  b, err := proto.Marshal(m)
  if err != nil {
    return false, fmt.Errorf("verify() marshal error: %w", err)
  }
  return k.Verify(b, sig.Data)
}

func (s *Server) Run(ctx context.Context) {
  // Run will return when there are peers to connect to
  s.t.Run(ctx)

  s.l.Info("Completing initial sync")
  s.Sync(ctx)

  s.l.Info("Running server main loop")
  s.notify()
  go func() {
    defer close(s.updateChan)
    for {
      select {
      case tm := <-s.t.OnMessage():
        // Attempt to store the public key of the sender so we can later verify messages
        if err := s.s.SetPubKey(tm.Peer, tm.PubKey); err != nil {
          s.l.Error("SetPubKey: %v", err)
        }
        switch v := tm.Msg.(type) {
          case *pb.Completion:
            s.handleCompletion(tm.Peer, v, tm.Signature)
          case *pb.Record:
            s.handleRecord(tm.Peer, v, tm.Signature)
          case *pb.PeerStatus:
            s.handlePeerStatus(tm.Peer, v)
        }
      case <-s.syncTicker.C:
        if err := s.s.Cleanup(); err != nil {
          s.l.Error("Sync: %v", err)
          return
        }
        go s.Sync(ctx)
      case <- s.publishStatusTicker.C:
        go s.sendStatus()
      case <-ctx.Done():
        return
      }
      s.notify()
    }
  }()
}

func (s *Server) notify() {
  select {
  case s.updateChan<- struct{}{}:
  default:
  }
}

func (s *Server) handleCompletion(peer string, c *pb.Completion, sig *pb.Signature) {
  s.l.Error("TODO handleCompletion")
  // Ignore if neither approver nor completer is us
  // Or if we don't already have a completion matching this uuid
  //
  // If completer is trusted and timestamp is null, republish with our
  // own signature so that other workers know not to attempt it
}

func (s *Server) handleRecord(peer string, c *pb.Record, sig *pb.Signature) {
  s.l.Error("TODO handleRecord")
  // Reject record if r.approver already has MaxRecordsPerPeer
  // Or if peer is not r.approver or r.worker
  // Or if peer would put us over MaxPeers
  //
  // Otherwise accept. Work selection is handled by the wrapper
}

func (s *Server) handlePeerStatus(peer string, c *pb.PeerStatus) {
  s.l.Error("TODO handlePeerStatus")
  // Reject if peer would put us over MaxPeers
}
