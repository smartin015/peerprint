package storage 

import (
  "context"
  "testing"
  "fmt"
  "strings"
  "time"
  "sync"
  "github.com/google/uuid"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "google.golang.org/protobuf/proto"
)

func testingDB() *sqlite3 {
  if db, err := NewSqlite3(":memory:"); err != nil {
    panic(err)
  } else {
    db.SetId("self")
    return db
  }
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

func TestSignedRecordSetGet(t *testing.T) {
  db := testingDB()
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
  db := testingDB()
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

func summarizeCompletions(gg []*pb.SignedCompletion) string {
  result := []string{}
  for _, g := range gg {
    exp := "set"
    if g.Completion.Timestamp == 0 {
      exp = "unset"
    }
    result = append(result, fmt.Sprintf("%s: %s (ts %s)", g.Completion.Uuid, g.Completion.Completer, exp))
  }
  return strings.Join(result, ", ")
}

func completionSlice(db *sqlite3) ([]*pb.SignedCompletion, error) {
  ch := make(chan *pb.SignedCompletion)
  got := []*pb.SignedCompletion{}
  go func() {
    for sc := range ch {
      got = append(got, sc)
    }
  }()
  if err := db.GetSignedCompletions(context.Background(), ch); err != nil {
    return nil, fmt.Errorf("GetSignedCompletions() = %v, want nil", err)
  }
  return got, nil
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
        CompleterState: []byte("testing"),
        Timestamp: ts,
      },
  }
  if err := db.SetSignedCompletion(sc); err != nil {
    panic(err)
  }
  return sc
}

func TestSignedCompletionsSetGet(t *testing.T) {
  db := testingDB()
  if got, err := cmpGet(db); err != nil || len(got) > 0 {
    t.Errorf("GetSignedCompletions = %v, %v want len() == 0, nil", got, err)
  }
  var want *pb.SignedCompletion
  for i := 0; i < 2; i++ {
    want = mustAddSC(db, fmt.Sprintf("signer%d", i), fmt.Sprintf("completer%d", i))
  }
  // Restrict signer lookup
  if got, err := cmpGet(db, WithSigner("signer1")); err != nil || len(got) != 1 || !proto.Equal(got[0], want) {
    t.Errorf("Get: %v, %v, want %v, nil", got, err, want)
  }
}
