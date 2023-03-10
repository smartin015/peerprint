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
  MaxTrackedPeers int64
}

type Interface interface {
  ID() string
  ShortID() string
  RegisterEventCallback(cb EventCallback)
  SetStatus(ps *pb.ClientStatus, publish bool) error

  IssueRecord(r *pb.Record, publish bool) (*pb.SignedRecord, error)
  IssueCompletion(g *pb.Completion, publish bool) (*pb.SignedCompletion, error)
  Sync(context.Context)

  Run(context.Context)

  GetSummary() *Summary
}

type EventCallback chan<-*pb.Event

type Server struct {
  opts *Opts
  t transport.Interface
  s storage.Interface
  l *log.Sublog
  cb []EventCallback

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
    cb: []EventCallback{},
    opts: opts,
    syncTicker: time.NewTicker(opts.SyncPeriod),
    connStr: "Initializing",
    lastSyncStart: time.Unix(0,0),
    lastSyncEnd: time.Unix(0,0),
    lastMsg: time.Unix(0,0),
  }
  if err := t.Register(PeerPrintProtocol, srv.getService()); err != nil {
    panic(fmt.Errorf("Failed to register RPC server: %w", err))
  }
  return srv
}

func (s *Server) ID() string {
  return s.t.ID()
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
  sig, err := s.t.Sign(g)
  if err != nil {
    return nil, fmt.Errorf("sign completion: %w", err)
  }
  sg := &pb.SignedCompletion{
    Completion: g,
    Signature: &pb.Signature{Signer: s.t.ID(), Data: sig},
  }
  s.l.Info("issueCompletion(%s)", pretty(sg))
  if err := s.s.SetSignedCompletion(sg); err != nil {
    return nil, fmt.Errorf("store completion: %w", err)
  }
  if publish {
    if err := s.t.Publish(DefaultTopic, sg); err != nil {
      return nil, fmt.Errorf("publish completion: %w", err)
    }
  }
  return sg, nil
}

func (s *Server) IssueRecord(r *pb.Record, publish bool) (*pb.SignedRecord, error) {
  s.l.Info("issueRecord(%s)", pretty(r))
  sig, err := s.t.Sign(r)
  if err != nil {
    return nil, fmt.Errorf("sign record: %w", err)
  }
  sr := &pb.SignedRecord{
    Record: r,
    Signature: &pb.Signature{Signer: s.t.ID(), Data: sig},
  }
  if err := s.s.SetSignedRecord(sr); err != nil {
    return nil, fmt.Errorf("write record: %w", err)
  }
  if publish {
    if err := s.t.Publish(DefaultTopic, sr); err != nil {
      return nil, fmt.Errorf("publish record: %w", err)
    }
  }
  return sr, nil
}

func (s *Server) syncRecords(ctx context.Context, p peer.ID) int {
  req := make(chan string, 1)
  req <- s.ID()
  close(req)
  rep := make(chan *pb.SignedRecord, 5) // Closed by Stream
  n := 0
  go func () {
    if err := s.t.Stream(ctx, p, "GetSignedRecords", req, rep); err != nil {
      s.l.Error("syncRecords(): GetSignedRecords(): %+v", err)
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
      if ok, err := s.t.Verify(v.Record, v.Signature.Signer, v.Signature.Data); err != nil {
        s.l.Warning("syncRecords() Verify(): %v", err)
        continue
      } else if !ok {
        s.l.Warning("syncRecords(): ignored (invalid signature) %s", pretty(v))
        continue
      }
      // Only accept records that are signed by this peer and where
      // the approver and signer are both the peer. This prevents
      // us from blindly trusting our peer to tell us about other peers.
      // Also accept our own signed records for data backfill purposes
      if (v.Signature.Signer != p.String() && v.Signature.Signer != s.ID()) || v.Record.Approver != v.Signature.Signer {
        s.l.Warning("syncRecords(): mismatch peer/signer/approver: %s/%s/%s", shorten(p.String()), shorten(v.Signature.Signer), shorten(v.Record.Approver))
      }

      // TODO should we prevent setting signed records if the created timestamp is earlier than the current version?

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
  req := make(chan string, 1)
  req <- s.ID()
  close(req)
  rep := make(chan *pb.SignedCompletion, 5) // Closed by Stream
  n := 0
  go func () {
    if err := s.t.Stream(ctx, p, "GetSignedCompletions", req, rep); err != nil {
      s.l.Error("syncCompletions() GetSignedCompletions(): %+v", err)
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
      if ok, err := s.t.Verify(v.Completion, v.Signature.Signer, v.Signature.Data); err != nil {
        s.l.Warning("syncCompletions() Verify(): %v", err)
        continue
      } else if !ok {
        s.l.Warning("syncCompletions(): ingnoring completion %s with invalid signature", pretty(v))
        continue
      }
      // Only accept completions that are signed by the peer and where
      // the completer and signer match. This prevents
      // us from blindly trusting our peer to tell us about other peers.
      // Self-issued completions signed by us are also accepted to
      // allow for data recovery.
      if (v.Signature.Signer != p.String() && v.Signature.Signer != s.ID()) || v.Completion.Completer != v.Signature.Signer {
        s.l.Warning("syncCompletions(): peer %s passed record with non-matching signer/completer %s/%s", shorten(p.String()), shorten(v.Signature.Signer), shorten(v.Completion.Completer))
        continue
      }

      // Ensure completion is within limits and has a matching source record
      if _, err := s.s.ValidateCompletion(v.Completion, v.Signature.Signer, s.opts.MaxTrackedPeers); err != nil {
        s.l.Warning("syncCompletions(): %v", err)
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
    wg.Add(1)
    s.l.Info("Syncing records from %s", shorten(p.String()))
    go func(p peer.ID) {
      defer wg.Done()
      n := s.syncRecords(ctx, p)
      s.l.Info("Synced %d records from %s", n, shorten(p.String()))
    }(p)
  }
  wg.Wait()

  for _, p := range s.t.GetPeers() {
    if p.String() == s.ID() {
      continue // No self connection
    }
    wg.Add(1)
    s.l.Info("Syncing completions from %s", shorten(p.String()))
    go func(p peer.ID) {
      defer wg.Done()
      n := s.syncCompletions(ctx, p)
      s.l.Info("Synced %d completions from %s", n, shorten(p.String()))
    }(p)
  }
  wg.Wait()
}

func (s *Server) Run(ctx context.Context) {
  // Run will return when there are peers to connect to
  s.connStr = "Searching for peers"
  if err := s.t.Run(ctx); err != nil {
    s.l.Error("Transport init: %v", err)
  }

  s.l.Info("Completing initial sync")
  s.Sync(ctx)

  s.l.Info("Cleaning up DB")
  if num, err := s.s.Cleanup(0); err != nil {
    s.l.Error("Cleanup: %v", err)
  } else {
    s.l.Info("Cleaned up %d records", num)
  }

  s.l.Info("Running server main loop")
  s.notify(&pb.Event{Name:"mainloop"})
  if err := s.s.AppendEvent("server", "main loop entered"); err != nil {
    s.l.Error("enter main loop: %v", err)
  }
  go func() {
    for {
      select {
      case tm := <-s.t.OnMessage():
        s.lastMsg = time.Now()
        if err := s.s.TrackPeer(tm.Signature.Signer); err != nil {
          s.l.Error("TrackPeer: %v", err)
        }

        switch v := tm.Msg.(type) {
          case *pb.SignedCompletion:
            if err := s.handleCompletion(tm.Peer, v.Completion, v.Signature); err != nil {
              s.l.Error("handleCompletion(%s, _, _): %v", shorten(tm.Peer), err)
            }
          case *pb.SignedRecord:
            if err := s.handleRecord(tm.Peer, v.Record, v.Signature); err != nil {
              s.l.Error("handleRecord(%s, _, _): %v", shorten(tm.Peer), err)
            }
          case *pb.PeerStatus:
            if err := s.handlePeerStatus(tm.Peer, v); err != nil {
              s.l.Error("handlePeerStatus(%s, _): %v", shorten(tm.Peer), err)
            }
        }
        s.notify(&pb.Event{Name:"message"})
      case <-s.syncTicker.C:
        if num, err := s.s.Cleanup(s.opts.MaxRecordsPerPeer/2); err != nil {
          s.l.Error("Sync cleanup: %v", err)
        } else {
          s.l.Info("Cleaned up %d records", num)
        }
        go s.Sync(ctx)
        s.notify(&pb.Event{Name:"sync"})
      case <-ctx.Done():
        return
      }
    }
  }()
}


func (s *Server) RegisterEventCallback(cb EventCallback) {
  s.cb = append(s.cb, cb)
}

func (s *Server) notify(m *pb.Event) {
  for _, cb := range s.cb {
    select {
    case cb<- m:
    default:
    }
  }
}

func (s *Server) handleCompletion(peer string, c *pb.Completion, sig *pb.Signature) error {
  if ok, err := s.t.Verify(c, sig.Signer, sig.Data); err != nil {
    return err
  } else if !ok {
    return fmt.Errorf("invalid signature %s", pretty(c))
  }

  sr, err := s.s.ValidateCompletion(c, peer, s.opts.MaxTrackedPeers)
  if err != nil {
    return fmt.Errorf("handleCompletion validation: %w", err)
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
    return nil
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

    // We can dump all other "speculative" completions as we assume the signer
    // is truthful about the state of their own record's completion
    // (they have no incentive to lie)
    if err := s.s.CollapseCompletions(c.Uuid, peer); err != nil {
      return fmt.Errorf("CollapseCompletions: %w", err)
    }
    s.notify(&pb.Event{
      Name:"progress",
      EventDetails: &pb.Event_Progress{Progress: &pb.Progress{
        Uuid: c.Uuid,
        ResolvedCompleter: c.Completer,
        Completed: c.Timestamp != 0,
      }},
    })
    return nil
  } else {
    s.l.Info("Overheard peer (%s) working on approver (%s)'s record (%s) (ts %d)", shorten(peer), shorten(sr.Record.Approver), c.Uuid, c.Timestamp)
    return nil
  }
}

func (s *Server) handlePeerStatus(peer string, ps *pb.PeerStatus) error {
  // This is noisy
  // s.l.Info("Received peer status for %s", shorten(peer))
  return s.s.SetPeerStatus(peer, ps)
}

func (s *Server) handleRecord(peer string, r *pb.Record, sig *pb.Signature) error {
  if ok, err := s.t.Verify(r, sig.Signer, sig.Data); err != nil {
    return err
  } else if !ok {
    return fmt.Errorf("invalid signature %s", pretty(r))
  }
  if err := s.s.ValidateRecord(r, peer, s.opts.MaxRecordsPerPeer, s.opts.MaxTrackedPeers); err != nil {
    return fmt.Errorf("handleRecord validation: %w", err)
  }
  s.l.Info("Received record: %s", pretty(r))
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

func (s *Server) SetStatus(status *pb.ClientStatus, publish bool) error {
  status.Timestamp = time.Now().Unix()
  ps := &pb.PeerStatus{
    Name: s.opts.DisplayName,
    Clients: []*pb.ClientStatus{status}, // merges into DB
  }
  if err := s.s.SetPeerStatus(s.ID(), ps); err != nil {
    return fmt.Errorf("save status: %w", err)
  }
  if publish {
    if err := s.t.Publish(DefaultTopic, ps); err != nil {
      return fmt.Errorf("publish status: %w", err)
    }
  }
  return nil
}
