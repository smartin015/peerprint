package storage 

import (
  "context"
  "testing"
  "fmt"
  "strings"
  "time"
  "sync"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "google.golang.org/protobuf/proto"
)

func testingDB() *sqlite3 {
  if db, err := NewSqlite3(":memory:", "self"); err != nil {
    panic(err)
  } else {
    return db
  }
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

func recGet(db *sqlite3, opts ...any) ([]*pb.SignedRecord, error) {
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

func TestSignedCompletionsSetGet(t *testing.T) {
  db := testingDB()
  if got, err := cmpGet(db); err != nil || len(got) > 0 {
    t.Errorf("GetSignedCompletions = %v, %v want len() == 0, nil", got, err)
  }

  want := &pb.SignedCompletion{
      Signature: &pb.Signature{
        Signer: "foo",
        Data: []byte{1,2,3},
      },
      Completion: &pb.Completion{
        Uuid: "foo",
        Completer: "peer1",
        Timestamp: time.Now().Unix(),
      },
  }
  for i := 0; i < 2; i++ {
    want.Signature.Signer = fmt.Sprintf("signer%d", i)
    want.Completion.Uuid = fmt.Sprintf("uuid%d", i)
    if err := db.SetSignedCompletion(want); err != nil {
      t.Errorf("SetSignedCompletion(%v): %v", want, err)
      return
    }
  }

  // Restrict signer lookup
  if got, err := cmpGet(db, WithSigner(want.Signature.Signer)); err != nil || len(got) != 1 || !proto.Equal(got[0], want) {
    t.Errorf("Get: %v, %v, want %v, nil", got, err, want)
  }

  // Lookup target peer1 including expired
  /*
  if gg, err := db.GetSignedCompletions(WithTarget("peer1"), IncludeExpired(true)); err != nil {
    t.Errorf("GetSignedCompletions(): %v", err)
  } else {
    got := summarizeCompletions(gg)
    want := "peer1: foo (live), peer1: bar (expired)"
    if got != want {
      t.Errorf("GetSignedCompletions() = %v, want %v", got, want)
    }
  }

  // Lookup peer1 excluding expired (default)
  if gg, err := db.GetSignedCompletions(WithTarget("peer1")); err != nil {
    t.Errorf("GetSignedCompletions(): %v", err)
  } else {
    got := summarizeCompletions(gg)
    want := "peer1: foo (live)"
    if got != want {
      t.Errorf("GetSignedCompletions() = %v, want %v", got, want)
    }
  }

  // Lookup 'baz' scope
  if gg, err := db.GetSignedCompletions(WithScope("baz")); err != nil {
    t.Errorf("GetSignedCompletions(): %v", err)
  } else {
    got := summarizeCompletions(gg)
    want := "peer2: baz (live)"
    if got != want {
      t.Errorf("GetSignedCompletions() = %v, want %v", got, want)
    }
  }

  // Lookup Editor type
  if gg, err := db.GetSignedCompletions(WithType(pb.CompletionType_EDITOR)); err != nil {
    t.Errorf("GetSignedCompletions(): %v", err)
  } else {
    got := summarizeCompletions(gg)
    want := "peer2: baz (live)" // Expired removed by default
    if got != want {
      t.Errorf("GetSignedCompletions() = %v, want %v", got, want)
    }
  }
  */
}

func TestComputePeerTrust(t *testing.T) {
  t.Errorf("TODO")
}

func TestComputeRecordWorkability(t *testing.T) {
  t.Errorf("TODO")
}

func TestCleanup(t *testing.T) {
  t.Errorf("TODO")
}
