package server

import (
  "testing"
  "fmt"
  "time"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
)

var (
  tNew = time.Now().Add(DefaultGrantTTL).Unix()
  tExpiring = time.Now().Add(GrantDeadlineAdvanceRefresh).Add(-1*time.Second).Unix()
  tExpired = time.Now().Add(-1*time.Second).Unix()
)

func testLeader() *leader {
  return &leader{
    base: &Server{
      t: transport.Fake("srv"),
      s: storage.NewInMemory(),
    },
    l: log.NewBasic(),
    ticker: nil,
  }
}

func expectNewAdmins(l *leader, admins []string) error {
  gg, err := l.base.s.GetSignedGrants()
  if err != nil {
    return fmt.Errorf("GetSignedGrants: %w", err)
  }

  want := make(map[string]struct{})
  for _, a := range admins {
    want[a] = struct{}{}
  }

  got := make(map[string]struct{})
  for _, g := range gg {
    if g.Grant.Type != pb.GrantType_ADMIN {
      continue
    } else if g.Grant.Expiry < time.Now().Unix() {
      continue
    } else if _, has := got[g.Grant.Target]; has {
      return fmt.Errorf("Multiple of same valid admin grant for target %s", g.Grant.Target)
    } else if _, has := want[g.Grant.Target]; !has {
      return fmt.Errorf("Target %s not in expected admins: %v", g.Grant.Target, admins)
    } else {
      got[g.Grant.Target] = struct{}{}
    }
  }

  if len(got) != len(want) {
    return fmt.Errorf("Want admins: %v, got %v", want, got)
  }
  return nil
}

func TestRefreshAdminGrantsEmpty(t *testing.T) {
  l := testLeader()
  if err := l.refreshAdminGrants([]*pb.SignedGrant{}); err != nil {
    t.Fatalf("refreshAdminGrants error: %s", err.Error())
  }
  if err := expectNewAdmins(l, []string{l.base.ID()}); err != nil {
    t.Fatalf(err.Error())
  }
}

func saGrant(signer, target string, expiry int64) *pb.SignedGrant {
  return &pb.SignedGrant{
    Signature: &pb.Signature{
      Signer: signer,
    },
    Grant: &pb.Grant{
      Target: target,
      Type: pb.GrantType_ADMIN,
      Expiry: expiry,
    },
  }
}

func TestRefreshAdminGrantsWithNewRoot(t *testing.T) {
  l := testLeader()
  if err := l.refreshAdminGrants([]*pb.SignedGrant{
    saGrant(l.base.ID(), l.base.ID(), tNew),
  }); err != nil {
    t.Fatalf("refreshAdminGrants error: %s", err.Error())
  }
  if err := expectNewAdmins(l, []string{}); err != nil {
    t.Fatalf(err.Error())
  }
}

func TestRefreshAdminGrantsWithExpiringRoot(t *testing.T) {
  l := testLeader()
  if err := l.refreshAdminGrants([]*pb.SignedGrant{
    saGrant(l.base.ID(), l.base.ID(), tExpiring),
  }); err != nil {
    t.Fatalf("refreshAdminGrants error: %s", err.Error())
  }
  if err := expectNewAdmins(l, []string{l.base.ID()}); err != nil {
    t.Fatalf(err.Error())
  }
}

func TestRefreshAdminGrantsWithExpiredRoot(t *testing.T) {
  l := testLeader()
  if err := l.refreshAdminGrants([]*pb.SignedGrant{
    saGrant(l.base.ID(), l.base.ID(), tExpired),
  }); err != nil {
    t.Fatalf("refreshAdminGrants error: %s", err.Error())
  }
  if err := expectNewAdmins(l, []string{l.base.ID()}); err != nil {
    t.Fatalf(err.Error())
  }
}

func TestRefreshAdminGrantsExpiringPeers(t *testing.T) {
  // Admin grants are renewed if they were issued by the leader
  // Admin grants are not renewed if they aren't traceable back to the leader
  // Admin grants are renewed if they follow the chain of authority back
  // to the leader
  l := testLeader()
  if err := l.refreshAdminGrants([]*pb.SignedGrant{
    saGrant(l.base.ID(), l.base.ID(), tNew),
    saGrant(l.base.ID(), "directPeer", tExpiring),
    saGrant("directPeer", "indirectPeer", tExpiring),
    saGrant("someOtherLeader", "orphanPeer", tExpiring),
  }); err != nil {
    t.Fatalf("refreshAdminGrants error: %s", err.Error())
  }
  if err := expectNewAdmins(l, []string{"directPeer", "indirectPeer"}); err != nil {
    t.Fatalf(err.Error())
  }
}

func TestRefreshAdminGrantsSkipsIfAnyNew(t *testing.T) {
  l := testLeader()
  if err := l.refreshAdminGrants([]*pb.SignedGrant{
    saGrant(l.base.ID(), l.base.ID(), tNew),
    saGrant(l.base.ID(), "directPeer", tNew),
    saGrant(l.base.ID(), "directPeer", tExpiring),
  }); err != nil {
    t.Fatalf("refreshAdminGrants error: %s", err.Error())
  }
  if err := expectNewAdmins(l, []string{}); err != nil {
    t.Fatalf(err.Error())
  }
}

func TestRefreshAdminGrantsIgnoresExpired(t *testing.T) {
  l := testLeader()
  if err := l.refreshAdminGrants([]*pb.SignedGrant{
    saGrant(l.base.ID(), l.base.ID(), tNew),
    saGrant(l.base.ID(), "directPeer", tExpired),
  }); err != nil {
    t.Fatalf("refreshAdminGrants error: %s", err.Error())
  }
  if err := expectNewAdmins(l, []string{}); err != nil {
    t.Fatalf(err.Error())
  }
}
