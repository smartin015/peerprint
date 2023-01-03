package server

import (
  "time"
  "context"
  pb "github.com/smartin015/peerprint/p2pgit/proto"
)

const (
  LeaderTimeout = 8 * time.Second
)

type electable struct {
  base *Server
  l *sublog
  watchdog *time.Timer
  syncTimer *time.Timer
}

func (s *electable) Init() {
  s.watchdog = time.NewTimer(LeaderTimeout)
  s.syncTimer = time.NewTimer(0)
}

func (s *electable) handlePeerStatus(peer string, ps *pb.PeerStatus) {
  if ps.Type == pb.PeerType_LEADER {
    s.watchdog.Reset(5 * time.Second)
    return
  }
}

// Electable's job is to keep an eye on whoever is currently leader, and to
// coordinate with other electable peers to elect a new leader if the current leader goes offline.
func (s *electable) Step(ctx context.Context) {
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
      case *pb.PeerStatus:
        s.handlePeerStatus(tm.Peer, v)
    }
  case <-s.watchdog.C:
    s.l.Info("Watchdog elapsed without leader status")
    s.base.handshake.Init()
  case <-s.syncTimer.C:
    s.syncTimer.Reset(SyncPeriod)
    s.base.partialSync()
  case <-ctx.Done():
    return
  }
}
