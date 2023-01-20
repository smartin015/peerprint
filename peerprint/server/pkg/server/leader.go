package server

import (
  "time"
  "fmt"
  "context"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
)

const (
  Heartbeat = 2 * time.Second
  DefaultGrantTTL = 5 * time.Minute
  GrantDeadlineAdvanceRefresh = 1*time.Minute
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

func (l *leader) Init() {
  l.base.changeRole(pb.PeerType_LEADER)
}

func (l *leader) Step(ctx context.Context) {
  select {
  case tm := <-l.base.t.OnMessage():
    // Attempt to store the public key of the sender so we can later verify messages
    if err := l.base.s.SetPubKey(tm.Peer, tm.PubKey); err != nil {
      l.l.Error("SetPubKey: %v", err)
    }
    switch v := tm.Msg.(type) {
      case *pb.Grant:
        l.handleGrantRequest(tm.Peer, v, tm.Signature)
      case *pb.Record:
        l.handleRecordRequest(tm.Peer, v, tm.Signature)
      case *pb.PeerStatus:
        l.handlePeerStatus(tm.Peer, v)
    }
  case <-l.ticker.C:
    grants, err := l.base.s.GetSignedGrants()
    if err != nil {
      l.l.Error("GetSignedGrants: %v", err)
    }
    if err := l.refreshAdminGrants(grants); err != nil {
      l.l.Error("refresh grants: %v", err)
    }
    if err := l.base.sendStatus(); err != nil {
      l.l.Error("sendStatus: %v", err)
    }
  case <-ctx.Done():
    return
  }
}

func (l *leader) handleGrantRequest(peer string, g *pb.Grant, sig *pb.Signature) error {
  // Never allow another peer to ask for ADMIN grant - leader makes that choice
  if g.Type == pb.GrantType_ADMIN {
    return fmt.Errorf("Rejecting admin grant request by peer %s", shorten(peer))
  }

  // Permit granting 
  if gg, err := l.base.s.GetSignedGrants(storage.WithScope(g.Scope)); err != nil {
    return fmt.Errorf("GetSignedGrants(scope=%s): %w", g.Scope, err)
  } else if len(gg) > 0 {
    return fmt.Errorf("Denying grant %v by peer %s - a grant with scope %s already exists", g, shorten(peer), g.Scope)
  } else {
    _, err := l.issueGrant(g)
    return err
  }
}

func (l *leader) handleRecordRequest(peer string, r *pb.Record, sig *pb.Signature) error {
  sr := &pb.SignedRecord{}
  if err := l.base.s.GetSignedRecord(r.Uuid, sr); err == nil {
    // If the record exists and is granted to this peer, allow it
    // Note that even "admin" granted users need an EDITOR grant to
    // edit a record, as these function similarly to locks in synchronized
    // parallel systems.
    if sg, err := l.base.s.GetSignedGrants(
      storage.WithScope(r.Uuid),
      storage.WithTarget(peer),
      storage.WithType(pb.GrantType_EDITOR),
    ); err != nil {
      return fmt.Errorf("GetSignedGrants: %w", err)
    } else if len(sg) == 0 {
      return fmt.Errorf("No grant exists for record %s", pretty(r))
      // TODO publish error to topic?
    } else {
      _, err := l.issueRecord(r)
      return err
    }
  } else if err == storage.ErrNoRows {
    // If the record doesn't exist, allow it
    _, err := l.issueRecord(r)
    return err
  } else {
    return fmt.Errorf("GetSignedRecord: %w", err)
  }
}

func (l *leader) issueRecord(r *pb.Record) (*pb.SignedRecord, error) {
  l.l.Info("issueRecord(%s)", pretty(r))
  sig, err := l.base.sign(r)
  if err != nil {
    return nil, fmt.Errorf("sign record: %w", err)
  }
  sr := &pb.SignedRecord{
    Record: r,
    Signature: sig,
  }
  if err := l.base.s.SetSignedRecord(sr); err != nil {
    return nil, fmt.Errorf("write record: %w", err)
  }
  if err := l.base.t.Publish(DefaultTopic, r); err != nil {
    return nil, fmt.Errorf("publish record: %w", err)
  }
  return sr, nil
}

func (l *leader) issueGrant(g *pb.Grant) (*pb.SignedGrant, error) {
  g.Expiry = time.Now().Add(DefaultGrantTTL).Unix()
  sig, err := l.base.sign(g)
  if err != nil {
    return nil, fmt.Errorf("sign grant: %w", err)
  }
  sg := &pb.SignedGrant{
    Grant: g,
    Signature: sig,
  }
  l.l.Info("issueGrant(%s)", pretty(sg))
  if err := l.base.s.SetSignedGrant(sg); err != nil {
    return nil, fmt.Errorf("store grant: %w", err)
  }
  if err := l.base.t.Publish(DefaultTopic, g); err != nil {
    return nil, fmt.Errorf("publish grant: %w", err)
  }
  return sg, nil
}

// refreshAdminGrants takes a list of grants and refreshes
// any grants that are about to expire.
// If there is no self-leadership grant, it is added
func (l *leader) refreshAdminGrants(grants []*pb.SignedGrant) error {
  // We wish to reauthorize grants for any admins that are about to expire
  // and that are directly or indirectly granted by us.
  // We can do this with a union-find of edges
  relAuth := NewUnionFind(l.base.ID())
  maxExpiry := NewMaxSet()
  admin := false
  now := time.Now().Unix()
  for _, g := range grants {
    if g.Grant.Type == pb.GrantType_ADMIN && g.Grant.Expiry > now {
      relAuth.Add(g.Grant.Target, g.Signature.Signer)
      maxExpiry.Add(g.Grant.Target, g.Grant.Expiry)
      admin = admin || g.Grant.Target == l.base.ID()
    }
  }

  // Build a new map so we only issue one grant for each target
  expiryThresh := time.Now().Add(GrantDeadlineAdvanceRefresh).Unix()
  renewals := make(map[string]struct{})
  for _, g := range grants {
    if g.Grant.Type != pb.GrantType_ADMIN || g.Grant.Expiry < now {
      continue
    }
    if _, ok := renewals[g.Grant.Target]; ok {
      continue // Already renewing
    }

    signer := relAuth.Find(g.Signature.Signer)
    exp := maxExpiry.Get(g.Grant.Target)
    if signer == l.base.ID() && exp < expiryThresh {
        renewals[g.Grant.Target] = struct{}{}
    }
  }

  for t, _ := range renewals {
    if _, err := l.issueGrant(&pb.Grant {
      Target: t,
      Type: pb.GrantType_ADMIN,
    }); err != nil {
      return fmt.Errorf("Reissue grant: %w", t, err)
    }
  }

  // Issue our own self-signed grant if not found
  if !admin {
    _, err := l.issueGrant(&pb.Grant {
      Target: l.base.t.ID(),
      Type: pb.GrantType_ADMIN,
    })
    return err
  }
  return nil
}


func (l *leader) handlePeerStatus(peer string, ps *pb.PeerStatus) {
  l.l.Info("handlePeerStatus of type %v", ps.Type)
  if ps.Type != pb.PeerType_UNKNOWN_PEER_TYPE {
    return
  }

  grants, err := l.base.s.GetSignedGrants()
  if err != nil {
    l.l.Info("Error fetching grants: %w", err)
    return
  }


  if na, err := l.base.s.CountAdmins(); err != nil {
    l.l.Info(fmt.Errorf("CountAdmins error: %w", err))
  } else if na < TargetAdminCount {
    if sg, err := l.issueGrant(&pb.Grant {
      Target: peer,
      Type: pb.GrantType_ADMIN,
    }); err != nil {
      l.l.Info(fmt.Errorf("Error issuing grant: %w", err))
    } else {
      grants = append(grants, sg)
    l.l.Info("Assigning ELECTABLE to %s (%d grants)", pretty(peer), len(grants))
      l.base.t.Publish(StatusTopic, &pb.AssignPeer {
        Peer: peer,
        Type: pb.PeerType_ELECTABLE,
        Grants: grants,
      })
    }
  } else {
    l.l.Info("Assigning LISTENER to %s (%d grants)", pretty(peer), len(grants))
    l.base.t.Publish(StatusTopic, &pb.AssignPeer {
      Peer: peer,
      Type: pb.PeerType_LISTENER,
      Grants: grants,
    })
  }
}

