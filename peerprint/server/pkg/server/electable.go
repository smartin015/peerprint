package server

import (
  "time"
  "context"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/log"
)

const (
  LeaderTimeout = 8 * time.Second
)

type electable struct {
  base *Server
  l *log.Sublog
  watchdog *time.Timer
  syncTimer *time.Timer
}

func (s *electable) Init() {
  s.base.changeRole(pb.PeerType_ELECTABLE)
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
      s.l.Info("OnMessage: %v", err)
    }
    switch v := tm.Msg.(type) {
      case *pb.Grant:
        s.base.storeGrantFromPeer(tm.Peer, v, tm.Signature)
      case *pb.Record:
        s.base.storeRecordFromPeer(tm.Peer, v, tm.Signature)
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
