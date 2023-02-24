package storage 

import (
  "testing"
  "fmt"
  "strings"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "google.golang.org/protobuf/proto"
)


func TestSignedRecordSetGet(t *testing.T) {
  db := testingDB(t)
  if got, err := recGet(db); err != nil || len(got) > 0 {
    t.Errorf("GetSignedRecord -> %v, %v want len()==0, nil", got, err)
    return
  }

  want := &pb.SignedRecord{
    Signature: &pb.Signature{
      Data: []byte{1,2,3},
    },
    Record: &pb.Record{
      Rank: &pb.Rank{Num:1, Den:1, Gen:1},
      Tags: []string{"foo", "bar", "baz"},
    },
  }
  for i := 0; i < 10; i++ {
    want.Record.Uuid = fmt.Sprintf("uuid%d", i)
    want.Signature.Signer = fmt.Sprintf("signer%d", i)
    if err := db.SetSignedRecord(want); err != nil {
      t.Errorf("Set: %v", err.Error())
    }
  }

  // Basic test
  if got, err := recGet(db); err != nil || len(got) != 10 {
    t.Errorf("Get: %v, %v, want len=10, nil", got, err)
  }
}

func TestSignedRecordNoTag(t *testing.T) {
  // Test for regressions if tag retrieval inserts additional rows
  db := testingDB(t)
  want := &pb.SignedRecord{
    Signature: &pb.Signature{
      Data: []byte{1,2,3},
      Signer: "foo",
    },
    Record: &pb.Record{
      Uuid: "asdf",
      Rank: &pb.Rank{Num:1, Den:1, Gen:1},
      Tags: []string{},
    },
  }
  if err := db.SetSignedRecord(want); err != nil {
    t.Errorf("Set: %v", err.Error())
  }
  if got, err := recGet(db); err != nil || len(got) != 1 || !proto.Equal(got[0], want) {
    t.Errorf("Get: %v, %v, want []{%v}, nil", got, err, want)
  }
}

func TestValidateRecordBadApprover(t *testing.T) {
  db := testingDB(t)
  sr := mustAddSR(db, "uuid", "signer")
  err := db.ValidateRecord(sr.Record, "peer", 100, 100)
  if !strings.Contains(err.Error(), "want peer") {
    t.Errorf("ValidateRecord: want approver mismatch, got %v", err)
  }
}

func TestValidateRecordOverMaxRecords(t *testing.T) {
  db := testingDB(t)
  sr := mustAddSR(db, "uuid", "peer")
  err := db.ValidateRecord(sr.Record, "peer", 0, 100)
  if !strings.Contains(err.Error(), "MaxRecordsPerPeer") {
    t.Errorf("ValidateRecord: want max records per peer error, got %v", err)
  }
}

func TestValidateRecordOverMaxTrackedPeers(t *testing.T) {
  db := testingDB(t)
  sr := mustAddSR(db, "uuid", "peer")
  err := db.ValidateRecord(sr.Record, "peer", 100, 0)
  if err == nil || !strings.Contains(err.Error(), "MaxTrackedPeers") {
    t.Errorf("ValidateRecord: want max tracked peer error, got %v", err)
  }
}

func TestValidateRecordPeerApprover(t *testing.T) {
  db := testingDB(t)
  sr := mustAddSR(db, "uuid", "peer")
  err := db.ValidateRecord(sr.Record, "peer", 100, 100)
  if err != nil {
    t.Errorf("ValidateRecord: want nil got %v", err)
  }
}

func TestValidateRecordSelfApprover(t *testing.T) {
  db := testingDB(t)
  sr := mustAddSR(db, "uuid", "self")
  err := db.ValidateRecord(sr.Record, "peer", 100, 100)
  if err != nil {
    t.Errorf("ValidateRecord: want nil got %v", err)
  }
}
