package storage 

import (
  "context"
  "testing"
  "fmt"
  "strings"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "google.golang.org/protobuf/proto"
)

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
  db := testingDB(t)
  if got, err := cmpGet(db); err != nil || len(got) > 0 {
    t.Errorf("GetSignedCompletions = %v, %v want len() == 0, nil", got, err)
  }
  var want *pb.SignedCompletion
  for i := 0; i < 2; i++ {
    want = mustAddSC(db, fmt.Sprintf("signer%d", i), fmt.Sprintf("completer%d", i))
  }
  // Restrict signer lookup
  if got, err := cmpGet(db, WithSigners([]{"signer1"})); err != nil || len(got) != 1 || !proto.Equal(got[0], want) {
    t.Errorf("Get: %v, %v, want %v, nil", got, err, want)
  }
}

func TestCollapseCompletions(t *testing.T) {
  db := testingDB(t)
  mustAddSCT(db, "cid", "signer1", "cpltr", true)
  mustAddSCT(db, "cid", "signer2", "cpltr", true)
  mustAddSCT(db, "cid", "signer3", "cpltr", true)
  if err := db.CollapseCompletions("cid", "signer1"); err != nil {
    t.Errorf("CollapseCompletions: %v", err)
  }
  if got, err := cmpGet(db); err != nil || len(got) != 1 {
    t.Errorf("CollapseCompletions -> len %d, %v; want 1, nil", len(got), err)
  }
}

func TestValidateCompletionNoRecord(t *testing.T) {
  db := testingDB(t)
  sc := mustAddSC(db, "signer", "cpltr")
  _, err := db.ValidateCompletion(sc.Completion, "peer", 100)
  if !strings.Contains(err.Error(), "no record") {
    t.Errorf("ValidateCompletion: want no record err, got %v", err)
  }
}

func TestValidateCompletionOverMaxPeers(t *testing.T) {
  db := testingDB(t)
  mustAddSR(db, "uuid", "signer")
  sc := mustAddSCT(db, "uuid", "cpltr", "cpltr", true)
  _, err := db.ValidateCompletion(sc.Completion, "peer", 0)
  if !strings.Contains(err.Error(), "MaxTrackedPeers") {
    t.Errorf("ValidateCompletion: want max tracked peers err, got %v", err)
  }
}

func TestValidateCompletionOK(t *testing.T) {
  db := testingDB(t)
  mustAddSR(db, "uuid", "signer")
  sc := mustAddSCT(db, "uuid", "cpltr", "cpltr", true)
  _, err := db.ValidateCompletion(sc.Completion, "peer", 1)
  if err != nil {
    t.Errorf("ValidateCompletion: want nil got %v", err)
  }
}
