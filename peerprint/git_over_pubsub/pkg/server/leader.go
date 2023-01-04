package server

import (
  "time"
  "fmt"
  "context"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/log"
)

const (
  Heartbeat = 2 * time.Second
)

// Leader's job is to
// - pass out Grants to peers that want to modify existing Records
// - Refresh the Grants of all Electable peers (and self)
// - Periodically publish a PeerStatus so everyone knows the leader is alive
// - Coordinate with other leaders in the event of a network partition
type leader struct {
  base *Server
  l *log.Sublog
  ticker *time.Ticker
}

func (s *leader) Init() {
  s.base.changeRole(pb.PeerType_LEADER)
}

func (s *leader) Step(ctx context.Context) {
  select {
  case tm := <-s.base.t.OnMessage():
    s.l.Info("%v", tm.Msg)
    // Attempt to store the public key of the sender so we can later verify messages
    if err := s.base.s.SetPubKey(tm.Peer, tm.PubKey); err != nil {
      s.l.Error("SetPubKey: %w", err)
    }
    switch v := tm.Msg.(type) {
      case *pb.Grant:
        s.base.storeGrant(tm.Peer, v, tm.Signature)
      case *pb.Record:
        s.base.storeRecord(tm.Peer, v, tm.Signature)
      case *pb.PeerStatus:
        s.handlePeerStatus(tm.Peer, v)
    }
  case <-s.ticker.C:
    if err := s.refreshAdminGrants(); err != nil {
      s.l.Error("refresh grants: %w", err)
    }
    if err := s.base.sendStatus(); err != nil {
      s.l.Error("sendStatus: %w", err)
    }
  case <-ctx.Done():
    return
  }
}

func (s *leader) issueGrant(g *pb.Grant) (*pb.SignedGrant, error) {
  g.Expiry = time.Now().Add(DefaultGrantTTL).Unix()
  sig, err := s.base.sign(g)
  if err != nil {
    return nil, fmt.Errorf("Self-sign grant: %w", err)
  }
  sg := &pb.SignedGrant{
    Grant: g,
    Signature: sig,
  }
  if err := s.base.s.SetSignedGrant(sg); err != nil {
    return nil, fmt.Errorf("write self grant: %w", err)
  }
  if err := s.base.t.Publish(DefaultTopic, g); err != nil {
    return nil, fmt.Errorf("publish self grant: %w", err)
  }
  return sg, nil
}

func (s *leader) refreshAdminGrants() error {
  grants, err := s.base.s.GetSignedGrants()
  if err != nil {
    return fmt.Errorf("GetSignedGrants: %w", err)
  }

  admin := false
  for _, g := range grants {
    if g.Grant.Type != pb.GrantType_ADMIN {
      continue
    }
    if g.Signature.Signer == s.base.t.ID() {
      g.Grant.Expiry = time.Now().Add(DefaultGrantTTL).Unix()
      if err := s.base.s.SetSignedGrant(g); err != nil {
        return fmt.Errorf("Extend grant %+v: %w", g.Grant, err)
      }
      if g.Grant.Target == s.base.t.ID() {
        admin = true
      }
    }
  }

  // Reapply self-signed grant if not found
  if !admin {
    s.l.Info("Issuing self grant")
    _, err := s.issueGrant(&pb.Grant {
      Target: s.base.t.ID(),
      Type: pb.GrantType_ADMIN,
    })
    return err
  }
  return nil
}


func (s *leader) handlePeerStatus(peer string, ps *pb.PeerStatus) {
  s.l.Info("handlePeerStatus of type %v", ps.Type)
  if ps.Type != pb.PeerType_UNKNOWN_PEER_TYPE {
    return
  }

  grants, err := s.base.s.GetSignedGrants()
  if err != nil {
    s.l.Info("Error fetching grants: %w", err)
    return
  }


  s.l.Info("Checking if we should grant admin")
  if na, err := s.base.s.CountAdmins(); err != nil {
    s.l.Info(fmt.Errorf("CountAdmins error: %w", err))
  } else if na < TargetAdminCount {
    if sg, err := s.issueGrant(&pb.Grant {
      Target: peer,
      Type: pb.GrantType_ADMIN,
    }); err != nil {
      s.l.Info(fmt.Errorf("Error issuing grant: %w", err))
    } else {
      grants = append(grants, sg)
    s.l.Info("Assigning ELECTABLE to %s (%d grants)", peer, len(grants))
      s.base.t.Publish(StatusTopic, &pb.AssignPeer {
        Peer: peer,
        Type: pb.PeerType_ELECTABLE,
        Grants: grants,
      })
    }
  } else {
    s.l.Info("Assigning LISTENER to %s (%d grants)", peer, len(grants))
    s.base.t.Publish(StatusTopic, &pb.AssignPeer {
      Peer: peer,
      Type: pb.PeerType_LISTENER,
      Grants: grants,
    })
  }
}

