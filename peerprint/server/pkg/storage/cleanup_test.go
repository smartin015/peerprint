package storage

import (
  "testing"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "fmt"
)

func TestCleanupNoRows(t *testing.T) {
  s := testingDB()
  if got, err := s.Cleanup(0); err != nil || got != 0 {
    t.Errorf("got %v, %v want 0, nil", got, err)
  }
}

func cplRec(s *sqlite3, num int64) {
  for i := int64(0); i < num; i++ {
    if err := s.SetSignedRecord(&pb.SignedRecord{
      Record: &pb.Record{
        Uuid: fmt.Sprintf("r%d", i),
        Approver: "foo", // Must equal signer
        Rank: &pb.Rank{},
      },
      Signature: &pb.Signature{
        Signer: "foo",
        Data: []byte("123"),
      },
    }); err != nil {
      panic(err)
    }
    if err := s.SetSignedCompletion(&pb.SignedCompletion{
      Completion: &pb.Completion{
        Uuid: fmt.Sprintf("r%d", i),
        Timestamp: 100+ i,
        CompleterState: []byte("foo"),
      },
      Signature: &pb.Signature{
        Signer: "foo",
        Data: []byte("123"),
      },
    }); err != nil {
      panic(err)
    }
  }
}

func TestCleanupWithinRange(t *testing.T) {
  s := testingDB()
  cplRec(s, 10)

  if got, err := s.Cleanup(10); err != nil || got != 0 {
    t.Errorf("got %v, %v want 0, nil", got, err)
  }
  if got, err := recGet(s); err != nil || len(got) != 10 {
    t.Errorf("GetSignedRecords = %v, %v, want len=10, nil", got, err)
  }
}

func TestCleanupOverRange(t *testing.T) {
  s := testingDB()
  cplRec(s, 10)

  if got, err := s.Cleanup(5); err != nil || got != 10 {
    // We want 10 records as cleanup clears both the records and completions
    // due to cascade deletion
    t.Errorf("got %v, %v want 10, nil", got, err)
  }
  if got, err := recGet(s); err != nil || len(got) != 5 {
    t.Errorf("GetSignedRecords = %v, %v, want len=5, nil", got, err)
  }
}

func TestCleanupDanglingCompletions(t *testing.T) {
  s := testingDB()
  if err := s.SetSignedCompletion(&pb.SignedCompletion{
    Completion: &pb.Completion{
      Uuid: "r0",
      Timestamp: 100,
      CompleterState: []byte("foo"),
    },
    Signature: &pb.Signature{
      Signer: "foo",
      Data: []byte("123"),
    },
  }); err != nil {
    t.Fatal(err)
  }
  if got, err := s.Cleanup(0); err != nil || got != 1 {
    t.Errorf("got %v, %v; want 1, nil", got, err)
  }
  if got, err := cmpGet(s); err != nil || len(got) != 0 {
    t.Errorf("GetSignedRecords = %v, %v, want len=0, nil", got, err)
  }
}
