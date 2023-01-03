package server

import (
  "time"
  "math/rand"
  "context"
  pb "github.com/smartin015/peerprint/p2pgit/proto"
)

const (

  StatusTopic = "_STATUS"

  // Maximum number of peers to add into the electorate for continuity when
  // the leader goes offline.
  MaxElectorate = 5

  // HandshakeStatusPeriodBase is the average handshake message period;
  // the actual period will be somewhere between 0.5x and 1.5x this value.
  HandshakeStatusPeriodBase = 2 * time.Second
)

type handshake struct {
  base *Server
  l *sublog
  accessionDelay time.Duration
  accessionTime time.Time
  lastBetterLeader time.Time
  tmr *time.Timer
}

func (s *handshake) betterLeaderFound(peer string, v *pb.PeerStatus) bool {
  return v.Type == pb.PeerType_UNKNOWN_PEER_TYPE && s.base.t.ID() < peer
}

func (s *handshake) tryAssign(v *pb.AssignPeer) bool {
  if v.Peer == s.base.t.ID() {
    for _, sg := range v.Grants {
      if err := s.base.s.SetSignedGrant(sg); err != nil {
        s.l.Info("Couldn't store grant %+v: %w", sg, err)
      } else {
        s.l.Info("Stored grant from %s", sg.Signature.Signer)
      }
    }
    s.base.status.Type = v.Type
    switch v.Type {
      case pb.PeerType_ELECTABLE:
        s.base.electable.Init()
      case pb.PeerType_LISTENER:
        s.base.listener.Init()
    }
    s.l.Info("Assignment succeeded; type now %v", s.base.status.Type)
  }
  return s.base.status.Type != pb.PeerType_UNKNOWN_PEER_TYPE
}

func (s *handshake) Init() {
  s.accessionTime = time.Now().Add(s.accessionDelay)
  s.lastBetterLeader = time.UnixMilli(0)
  s.tmr = time.NewTimer(0)
  s.base.status.Type = pb.PeerType_UNKNOWN_PEER_TYPE
  s.l.Info("Beginning handshake loop; accession in %v (at %v)\n", s.accessionDelay, s.accessionTime)
  s.base.sendStatus()
}

func (s *handshake) Step(ctx context.Context) {
  select {
  case m := <-s.base.t.OnMessage():
    if m.Topic == StatusTopic {
      switch v := m.Msg.(type) {
      case *pb.PeerStatus:
        if s.betterLeaderFound(m.Peer, v) {
          s.l.Info("Found better leader candidate: %s", m.Peer)
          s.lastBetterLeader = time.Now()
        }
      case *pb.AssignPeer:
        if s.tryAssign(v) {
          return
        }
      }
    }
  case <-s.tmr.C:
    if time.Now().After(s.accessionTime) && s.lastBetterLeader.Before(time.Now().Add(-s.accessionDelay)) {
      s.l.Info("accession period elapsed with no better peer; assuming leadership")
      s.base.leader.Init()
      return
    } else {
      s.base.sendStatus()
      millis := (0.5 + rand.Float64()) * float64(HandshakeStatusPeriodBase.Milliseconds())
      nextStatus := time.Duration(millis) * time.Millisecond
      s.tmr = time.NewTimer(nextStatus)
      s.l.Info("Sent status; next attempt in %v", nextStatus)
    }
  }
}
