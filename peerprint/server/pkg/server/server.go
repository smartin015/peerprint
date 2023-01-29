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
  UpdateChannelBufferSize = 20
)

type Opts struct {
  SyncPeriod time.Duration
  DisplayName string

  MaxRecordsPerPeer int64
  MaxCompletionsPerPeer int64
  MaxTrackedPeers int64

  TrustedWorkerThreshold float64
}

type Interface interface {
  ID() string
  ShortID() string
  GetService() interface{}

  IssueRecord(r *pb.Record, publish bool) (*pb.SignedRecord, error)
  IssueCompletion(g *pb.Completion, publish bool) (*pb.SignedCompletion, error)
  Sync(context.Context)

  Run(context.Context)
  OnUpdate() <-chan proto.Message

  GetSummary() *Summary
}

type Server struct {
  opts *Opts
  t transport.Interface
  s storage.Interface
  l *log.Sublog

  updateChan chan proto.Message

  // Tickers for periodic network activity
  syncTicker *time.Ticker

  connStr string // Indicate stage of connection
  lastSyncStart time.Time
  lastSyncEnd time.Time
  lastMsg time.Time
}

func New(t transport.Interface, s storage.Interface, opts *Opts, l *log.Sublog) *Server {
  srv := &Server{
    t: t,
    s: s,
    l: l,
    opts: opts,
    updateChan: make(chan proto.Message, UpdateChannelBufferSize),
    syncTicker: time.NewTicker(opts.SyncPeriod),
    connStr: "Initializing",
    lastSyncStart: time.Unix(0,0),
    lastSyncEnd: time.Unix(0,0),
    lastMsg: time.Unix(0,0),
  }
  if err := t.Register(PeerPrintProtocol, srv.GetService()); err != nil {
    panic(fmt.Errorf("Failed to register RPC server: %w", err))
  }
  return srv
}

func (s *Server) ID() string {
  return s.t.ID()
}


func (s *Server) OnUpdate() <-chan proto.Message {
  return s.updateChan
}

func pretty(i interface{}) string {
  switch v := i.(type) {
  case *pb.SignedCompletion:
    return fmt.Sprintf("Completion{signer %s: %s did %s}", shorten(v.Signature.Signer), shorten(v.Completion.Completer), v.Completion.Uuid)
  case *pb.Completion:
    return fmt.Sprintf("Completion{%s did %s}", shorten(v.Completer), v.Uuid)
  case *pb.Record:
    return fmt.Sprintf("Record{%s->%s (%d tags)}", shorten(v.Uuid), shorten(v.Manifest), len(v.Tags))
  case *pb.SignedRecord:
    return fmt.Sprintf("Record{signer %s: %s->%s (%d tags)}", shorten(v.Signature.Signer), shorten(v.Record.Uuid), shorten(v.Record.Manifest), len(v.Record.Tags))
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

func (s *Server) syncRecords(ctx context.Context, p peer.ID) int {
  req := make(chan struct{}); close(req)
  rep := make(chan *pb.SignedRecord, 5) // Closed by Stream
  n := 0
  go func () {
    defer storage.HandlePanic()
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
      // Ensure signature is valid
      if ok, err := s.verify(v.Record, v.Signature); err != nil {
        s.l.Warning("syncRecords() verify(): %v", err)
        continue
      } else if !ok {
        s.l.Warning("syncRecords(): ingnoring record %s with invalid signature", pretty(v))
      }
      // Only accept records that are signed by this peer and where
      // the approver and signer are both the peer. This prevents
      // us from blindly trusting our peer to tell us about other peers.
      if v.Signature.Signer != p.String() || v.Record.Approver != v.Signature.Signer {
        s.l.Warning("syncRecords(): peer %s passed record with non-matching signer/approver %s/%s", shorten(p.String()), shorten(v.Signature.Signer), shorten(v.Record.Approver))
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
    defer storage.HandlePanic()
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
      // Ensure signature is valid
      if ok, err := s.verify(v.Completion, v.Signature); err != nil {
        s.l.Warning("syncCompletions() verify(): %v", err)
        continue
      } else if !ok {
        s.l.Warning("syncCompletions(): ingnoring completion %s with invalid signature", pretty(v))
      }
      // Only accept completions that are signed by this peer and where
      // the completer and signer are both the peer. This prevents
      // us from blindly trusting our peer to tell us about other peers.
      if v.Signature.Signer != p.String() || v.Completion.Completer != v.Signature.Signer {
        s.l.Warning("syncCompletions(): peer %s passed record with non-matching signer/completer %s/%s", shorten(p.String()), shorten(v.Signature.Signer), shorten(v.Completion.Completer))
        continue
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
  s.connStr = "Syncing"
  s.lastSyncStart = time.Now()
  s.lastSyncEnd = time.Unix(0,0)
  defer func() {
    s.connStr = "Synced"
    s.lastSyncEnd = time.Now()
  }()
  var wg sync.WaitGroup
  for _, p := range s.t.GetPeers() {
    if p.String() == s.ID() {
      continue // No self connection
    }
    // TODO fetch <= MaxRecordsPerPeer records and MaxCompletionsPerPeer completions for p
    wg.Add(2)
    s.l.Info("Syncing with %s", shorten(p.String()))
    go func(p peer.ID) {
      defer storage.HandlePanic()
      defer wg.Done()
      n := s.syncRecords(ctx, p)
      s.l.Info("Synced %d records from %s", n, shorten(p.String()))
    }(p)
    go func(p peer.ID) {
      defer storage.HandlePanic()
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
  pid, err := peer.Decode(sig.Signer)
  if err != nil {
    return false, fmt.Errorf("verify() decode ID: %w", err)
  }
  k, err := pid.ExtractPublicKey()
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
  s.connStr = "Searching for peers"
  s.t.Run(ctx)

  s.l.Info("Completing initial sync")
  s.Sync(ctx)

  s.l.Info("Cleaning up DB")
  for _, err := range s.s.Cleanup(s.opts.MaxTrackedPeers) {
    s.l.Error("Cleanup: %v", err)
  }

  s.l.Info("Running server main loop")
  s.notify(&pb.NotifyReady{})
  if err := s.s.AppendEvent("server", "main loop entered"); err != nil {
    s.l.Error("enter main loop: %v", err)
  }
  go func() {
    defer storage.HandlePanic()
    defer close(s.updateChan)
    for {
      select {
      case tm := <-s.t.OnMessage():
        s.lastMsg = time.Now()
        if err := s.s.TrackPeer(tm.Signature.Signer); err != nil {
          s.l.Error("TrackPeer: %v", err)
        }

        switch v := tm.Msg.(type) {
          case *pb.Completion:
            if err := s.handleCompletion(tm.Peer, v, tm.Signature); err != nil {
              s.l.Error("handleCompletion(%s, _, _): %v", shorten(tm.Peer), err)
            }
          case *pb.Record:
            if err := s.handleRecord(tm.Peer, v, tm.Signature); err != nil {
              s.l.Error("handleRecord(%s, _, _): %v", shorten(tm.Peer), err)
            }
        }
        s.notify(&pb.NotifyMessage{})
      case <-s.syncTicker.C:
         for _, err := range s.s.Cleanup(s.opts.MaxTrackedPeers) {
          s.l.Error("Sync cleanup: %v", err)
        }
        go s.Sync(ctx)
        s.notify(&pb.NotifySync{})
      case <-ctx.Done():
        return
      }
    }
  }()
}

func (s *Server) notify(m proto.Message) {
  select {
  case s.updateChan<- m:
  default:
  }
}

func (s *Server) handleCompletion(peer string, c *pb.Completion, sig *pb.Signature) error {
  // Ignore if there's no matching record
  sr, err := s.s.GetSignedSourceRecord(c.Uuid)
  if err != nil {
    return fmt.Errorf("GetSignedSourceRecord: %w", err)
  }
  // Reject record if peer already has MaxCompletionsPerPeer
  if num, err := s.s.CountSignerCompletions(peer); err != nil {
    return fmt.Errorf("CountSignerCompletions: %w", err)
  } else if num > s.opts.MaxCompletionsPerPeer {
    return fmt.Errorf("MaxCompletionsPerPeer exceeded (%d > %d)", num, s.opts.MaxCompletionsPerPeer)
  }
  // Or if peer would put us over MaxTrackedPeers
  if num, err := s.s.CountCompletionSigners(peer); err != nil {
    return fmt.Errorf("CountCompletionSigners: %w", err)
  } else if num > s.opts.MaxTrackedPeers {
    return fmt.Errorf("MaxTrackedPeers exceeded (%d > %d)", num, s.opts.MaxTrackedPeers)
  }

  sc := &pb.SignedCompletion{
    Completion: c,
    Signature: sig,
  }

  if err := s.s.SetSignedCompletion(sc); err != nil {
    return fmt.Errorf("SetSignedCompletion: %w", err)
  }

  if sr.Record.Approver == s.ID() && c.Timestamp == 0 {
    // Peer talking about a record of ours
    s.l.Info("Peer %s is working on our record (%s)", shorten(peer), c.Uuid)
    if trust, err := s.s.GetWorkerTrust(peer); err != nil {
      return fmt.Errorf("GetTrust: %w", err)
    } else if trust > s.opts.TrustedWorkerThreshold {
      // TODO prevent another trusted peer from coming along and swiping the work - must check if record already has a sponsored worker
      s.l.Info("We nominate %s to complete (%s)", shorten(peer), c.Uuid)
      _, err := s.IssueCompletion(c, true)
      return err
    } else {
      return nil
    }
  } else if sr.Record.Approver == peer && sr.Signature.Signer == peer {
    // Record owner is talking about completion of their record. Either
    // timestamp=0 in which case they acknowledge someone working on it,
    // or timestamp>0 in which case they acknowledge receipt of the physical
    // goods.
    if c.Timestamp != 0 {
      s.l.Info("Peer %s confirmed our completion (%s)", shorten(peer), c.Uuid)
    } else {
      s.l.Info("Peer %s sponsors completer %s", shorten(peer), shorten(c.Completer))
    }

    if err := s.s.SetSignedCompletion(sc); err != nil {
      return fmt.Errorf("SetSignedCompletion: %w", err)
    }
    // We can dump all other "speculative" completions as we assume the signer
    // is truthful about the state of their own record's completion
    // (they have no incentive to lie)
    if err := s.s.CollapseCompletions(c.Uuid, peer); err != nil {
      return fmt.Errorf("CollapseCompletions: %w", err)
    }
    s.notify(&pb.NotifyProgress{
      Uuid: c.Uuid,
      ResolvedCompleter: c.Completer,
      Completed: c.Timestamp != 0,
    })
    return nil
  } else {
    s.l.Info("Overheard peer (%s) completing approver (%s)'s record (%s) (ts %d)", shorten(peer), shorten(sr.Record.Approver), c.Uuid, c.Timestamp)
    return nil
  }
}

func (s *Server) handleRecord(peer string, r *pb.Record, sig *pb.Signature) error {
  // Reject if neither we nor the peer are the approver
  if r.Approver != peer && r.Approver != s.ID() {
    return fmt.Errorf("Approver=%s, want peer (%s) or self (%s)", shorten(r.Approver), shorten(peer), shorten(s.ID()))
  }
  // Reject record if peer already has MaxRecordsPerPeer
  if num, err := s.s.CountSignerRecords(peer); err != nil {
    return fmt.Errorf("CountSignerRecords: %w", err)
  } else if num > s.opts.MaxRecordsPerPeer {
    return fmt.Errorf("MaxRecordsPerPeer exceeded (%d > %d)", num, s.opts.MaxRecordsPerPeer)
  }
  // Or if peer would put us over MaxTrackedPeers
  if num, err := s.s.CountRecordSigners(peer); err != nil {
    return fmt.Errorf("CountRecordSigners: %w", err)
  } else if num > s.opts.MaxTrackedPeers {
    return fmt.Errorf("MaxTrackedPeers exceeded (%d > %d)", num, s.opts.MaxTrackedPeers)
  }
  // Otherwise accept. Note that we just store it here; work selection is handled out of band (via the wrapper)
  return s.s.SetSignedRecord(&pb.SignedRecord{
    Record: r,
    Signature: sig,
  })
}

type Summary struct {
  ID string
  Peers []*peer.AddrInfo
  Connection string
  LastSyncStart int64
  LastSyncEnd int64
  LastMessage int64
}

func (s *Server) GetSummary() *Summary {
  return &Summary{
    ID: s.ID(),
    Peers: s.t.GetPeerAddresses(),
    Connection: s.connStr,
    LastSyncStart: s.lastSyncStart.Unix(),
    LastSyncEnd: s.lastSyncEnd.Unix(),
    LastMessage: s.lastMsg.Unix(),
  }
}
