package storage 

import (
  "context"
  "testing"
  "sync"
  "time"
  "github.com/google/uuid"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
)

func testingDB(t *testing.T) *sqlite3 {
  if db, err := NewSqlite3(":memory:"); err != nil {
    t.Fatalf(err.Error())
    return nil
  } else {
    db.SetId("self")
    t.Cleanup(db.Close)
    return db
  }
}

func mustAddSR(db *sqlite3, uuid, signer string) *pb.SignedRecord {
  sr := &pb.SignedRecord{
      Signature: &pb.Signature{
        Signer: signer,
        Data: []byte{1,2,3},
      },
      Record: &pb.Record{
        Uuid: uuid,
        Approver: signer,
        Rank: &pb.Rank{},
      },
  }
  if err := db.SetSignedRecord(sr); err != nil {
    panic(err)
  }
  return sr
}


func pstatGet(db Interface, opts ...any) ([]*pb.PeerStatus, error) {
  got := []*pb.PeerStatus{}
  ch := make(chan *pb.PeerStatus)
  var wg sync.WaitGroup
  wg.Add(1)
  go func(){
    defer wg.Done()
    for sr := range ch {
      got = append(got, sr)
    }
  }()
  err := db.GetPeerStatuses(context.Background(), ch, opts...)
  wg.Wait()
  return got, err
}


func recGet(db Interface, opts ...any) ([]*pb.SignedRecord, error) {
  got := []*pb.SignedRecord{}
  ch := make(chan *pb.SignedRecord)
  var wg sync.WaitGroup
  wg.Add(1)
  go func(){
    defer wg.Done()
    for sr := range ch {
      got = append(got, sr)
    }
  }()
  err := db.GetSignedRecords(context.Background(), ch, opts...)
  wg.Wait()
  return got, err
}

func mustAddSC(db *sqlite3, signer, completer string) *pb.SignedCompletion {
  return mustAddSCT(db, uuid.New().String(), signer, completer, true)
}
func mustAddSCT(db *sqlite3, uuid, signer, completer string, with_timestamp bool) *pb.SignedCompletion {
  ts := int64(0)
  if with_timestamp {
    ts = time.Now().Unix()
  }
  sc := &pb.SignedCompletion{
      Signature: &pb.Signature{
        Signer: signer,
        Data: []byte{1,2,3},
      },
      Completion: &pb.Completion{
        Uuid: uuid,
        Completer: completer,
        Client: "testclient",
        Type: pb.CompletionType_ACQUIRE,
        CompleterState: []byte("testing"),
        Timestamp: ts,
      },
  }
  if err := db.SetSignedCompletion(sc); err != nil {
    panic(err)
  }
  return sc
}

func cmpGet(db *sqlite3, opts ...any) ([]*pb.SignedCompletion, error) {
  got := []*pb.SignedCompletion{}
  ch := make(chan *pb.SignedCompletion)
  var wg sync.WaitGroup
  wg.Add(1)
  go func(){
    defer wg.Done()
    for sr := range ch {
      got = append(got, sr)
    }
  }()
  err := db.GetSignedCompletions(context.Background(), ch, opts...)
  wg.Wait()
  return got, err
}

func TestGetSummary(t *testing.T) {
  db := testingDB(t)
  s, errs := db.GetSummary()
  for _, e := range errs {
    t.Errorf("GetSummary: %v", e)
  }
  if s == nil {
    t.Errorf("Want summary, got nil")
  }
}

