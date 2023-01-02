package server

import (
  "context"
  "time"
  pb "github.com/smartin015/peerprint/p2pgit/proto"
)

type listener struct {
  base *Server
  l *sublog
  syncTimer *time.Timer
}

func (s *listener) Init() {
  s.syncTimer = time.NewTimer(0)
}

// Listener's job is mostly just to watch messages go by and keep the 
// storage layer up to date with them.
// Also periodically syncing state with adjacent peers to make sure no changes
// were missed.
func (s *listener) Step(ctx context.Context) {
  select {
  case tm := <-s.base.t.OnMessage():
    // Attempt to store the public key of the sender so we can later verify messages
    if err := s.base.s.SetPubKey(tm.Peer, tm.PubKey); err != nil {
      s.l.Info("SetPubKey error: %w", err)
    }
    switch v := tm.Msg.(type) {
      case *pb.Grant:
        s.base.storeGrant(tm.Peer, v, tm.Signature)
      case *pb.Record:
        s.base.storeRecord(tm.Peer, v, tm.Signature)
    }
  case err := <-s.base.t.OnError():
    s.l.Error("Transport: %w", err)
  case <-s.syncTimer.C:
    s.syncTimer.Reset(SyncPeriod)
    s.base.partialSync()
  case <-ctx.Done():
    return
  }
}
