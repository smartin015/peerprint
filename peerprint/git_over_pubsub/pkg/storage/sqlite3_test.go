package storage 

import (
  "testing"
  "fmt"
  "strings"
  "database/sql"
  "time"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "google.golang.org/protobuf/proto"
)

func testingDB() *sqlite3 {
  if db, err := NewSqlite3(":memory:"); err != nil {
    panic(err)
  } else {
    return db
  }
}

func TestPubKeySetGet(t *testing.T) {
  db := testingDB()
  if _, err := db.GetPubKey("foo"); err == nil {
    t.Errorf("GetPubKey of peer without key should error")
  }

  _, pub, err := crypto.GenKeyPair()
  if err != nil {
    t.Fatalf(err.Error())
  }

  if err := db.SetPubKey("foo", pub); err != nil {
    t.Errorf("SetPubKey: %v", err)
  }
  if got, err := db.GetPubKey("foo"); err != nil || !got.Equals(pub) {
    t.Errorf("GetPubKey(foo) = %v, %v, want %v, nil", got, err, pub)
  }
}

func rSetGetEq(db *sqlite3, r *pb.SignedRecord, want *pb.SignedRecord) error {
  got := &pb.SignedRecord{}
  if err := db.SetSignedRecord(r); err != nil {
    return fmt.Errorf("SetSignedRecord: %v", err)
  }
  if err := db.GetSignedRecord(r.Record.Uuid, got); err != nil {
    if err == sql.ErrNoRows && want == nil {
      return nil
    }
    return fmt.Errorf("GetSignedRecord(): %v", err)
  } else if !proto.Equal(got, want) {
    return fmt.Errorf("GetSignedRecord(): got %v want %v", got, want)
  }
  return nil
}

func TestSignedRecordSetGet(t *testing.T) {
  db := testingDB()
  got := &pb.SignedRecord{}
  if err := db.GetSignedRecord("asdf", got); err == nil {
    t.Errorf("GetSignedRecord of nonexistant uuid should error")
    return
  }

  want := &pb.SignedRecord{
    Signature: &pb.Signature{
      Signer: "foo",
      Data: []byte{1,2,3},
    },
    Record: &pb.Record{
      Uuid: "uuid",
      Version: 1,
      Rank: &pb.Rank{Num:1, Den:1, Gen:1},
      Tags: []string{"foo", "bar", "baz"},
    },
  }
  // Basic test
  if err := rSetGetEq(db, want, want); err != nil {
    t.Errorf(err.Error())
  }

  // Always read latest version
  want.Record.Version = 2
  if err := rSetGetEq(db, want, want); err != nil {
    t.Errorf(err.Error())
  }

  // Don't fetch tombstoned version
  want.Record.Uuid = "uuid2"
  want.Record.Tombstone = time.Now().Unix()
  if err := rSetGetEq(db, want, nil); err != nil {
    t.Errorf(err.Error())
  }

  // GetSignedRecords() only returns non-tombstoned, latest version
  if rr, err := db.GetSignedRecords(); err != nil {
    t.Errorf(err.Error())
  } else if len(rr) != 1 {
    t.Errorf("Want exactly 1 record, got %d", len(rr))
  } else if r := rr[0].Record; r.Version != 2 {
    t.Errorf("Want latest version, got %d", r.Version)
  } else if r.Tombstone != 0 {
    t.Errorf("Want zero tombstones, got %d", r.Tombstone)
  }
}

func summarizeGrants(gg []*pb.SignedGrant) string {
  result := []string{}
  for _, g := range gg {
    exp := "live"
    if g.Grant.Expiry < time.Now().Unix() {
      exp = "expired"
    }
    result = append(result, fmt.Sprintf("%s: %s (%s)", g.Grant.Target, g.Grant.Scope, exp))
  }
  return strings.Join(result, ", ")
}

func TestSignedGrantsSetGet(t *testing.T) {
  db := testingDB()
  if got, err := db.GetSignedGrants(); err != nil || len(got) > 0 {
    t.Errorf("GetSignedGrants() = %v, %v, want empty, nil", got, err)
    return
  }
  sig := &pb.Signature{
    Signer: "foo",
    Data: []byte{1,2,3},
  }

  for _, g := range []*pb.SignedGrant{
    &pb.SignedGrant{
      Signature: sig,
      Grant: &pb.Grant{
        Target: "peer1",
        Expiry: time.Now().Add(10*time.Minute).Unix(),
        Scope: "foo",
      },
    },
    &pb.SignedGrant{
      Signature: sig,
      Grant: &pb.Grant{
        Target: "peer1",
        Expiry: time.Now().Add(-10*time.Minute).Unix(),
        Scope: "bar",
      },
    },
    &pb.SignedGrant{
      Signature: sig,
      Grant: &pb.Grant{
        Target: "peer2",
        Expiry: time.Now().Add(10*time.Minute).Unix(),
        Scope: "baz",
      },
    },
  } {
    if err := db.SetSignedGrant(g); err != nil {
      t.Errorf("SetSignedGrant(%v): %v", g, err)
      return
    }
  }

  // Lookup target peer1 including expired
  if gg, err := db.GetSignedGrants(WithTarget("peer1"), IncludeExpired(true)); err != nil {
    t.Errorf("GetSignedGrants(): %v", err)
  } else {
    got := summarizeGrants(gg)
    want := "peer1: foo (live), peer1: bar (expired)"
    if got != want {
      t.Errorf("GetSignedGrants() = %v, want %v", got, want)
    }
  }

  // Lookup peer1 excluding expired (default)
  if gg, err := db.GetSignedGrants(WithTarget("peer1")); err != nil {
    t.Errorf("GetSignedGrants(): %v", err)
  } else {
    got := summarizeGrants(gg)
    want := "peer1: foo (live)"
    if got != want {
      t.Errorf("GetSignedGrants() = %v, want %v", got, want)
    }
  }

  // Lookup 'baz' scope
  if gg, err := db.GetSignedGrants(WithScope("baz")); err != nil {
    t.Errorf("GetSignedGrants(): %v", err)
  } else {
    got := summarizeGrants(gg)
    want := "peer2: baz (live)"
    if got != want {
      t.Errorf("GetSignedGrants() = %v, want %v", got, want)
    }
  }
}

func TestIsAdmin(t *testing.T) {
  t.Errorf("TODO")
}

func TestCountAdmins(t *testing.T) {
  t.Errorf("TODO")
}

func TestValidGrants(t *testing.T) {
  t.Errorf("TODO")
}
