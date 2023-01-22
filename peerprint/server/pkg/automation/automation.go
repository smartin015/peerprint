package automation

import (
  "context"
  "log"
  "os"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "time"
  //"math/rand"
  //"fmt"
  "github.com/google/uuid"
  xrand "golang.org/x/exp/rand"
  "gonum.org/v1/gonum/stat/distuv"
)

var (
  logger = log.New(os.Stderr, "", 0)
)

func dlog(fmt string, args ...any) {
  // Magenta, then fmt string, then reset colors
  logger.Printf("\u001b[35m" + fmt + "\u001b[0m", args...)
}

type loadTester struct {
  targetRecords int64
  s storage.Interface
  qps float64
  successes int
  errors map[ErrorType]int
  srv server.Interface
}
func NewLoadTester(qps float64, targetRecords int64, srv server.Interface, s storage.Interface) *loadTester {
  return &loadTester{
    targetRecords: targetRecords,
    qps: qps,
    srv: srv,
    s: s,
    successes: 0,
    errors: make(map[ErrorType]int),
  }
}

func (l *loadTester) Run(ctx context.Context) {
  dlog("Will make random changes via Poisson distribution with a mean of %f QPS", l.qps)
  source := xrand.NewSource(uint64(time.Now().UnixNano()))
  poisson := distuv.Poisson{
    Lambda: 1.0/l.qps,
    Src:    source,
  }

  for {
    tmr := time.NewTimer(time.Duration(1000.0 * poisson.Rand()) * time.Millisecond)
    select {
    case <-tmr.C:
      go func() {
        typ, err := l.MakeRandomChange()
        if err != nil {
          l.errors[err.Type] += 1
          dlog("%24s -> %s", typ, err.Error())
        } else {
          l.successes += 1
          dlog("%24s -> ok", typ)
        }
      }()
    case <-ctx.Done():
      dlog("Finishing run")
      return
    }
  }
}


func (l *loadTester) addRecord() *LoadTestErr {
  r := &pb.Record {
    Uuid: uuid.New().String(),
    Tags: []string{},
    Location: uuid.New().String(),
    Created: time.Now().Unix(),
    Tombstone: 0,
    Rank: &pb.Rank{Num: 0, Den: 0, Gen: 0},
  }
  if err := l.srv.IssueRecord(r); err != nil {
    return derr(ErrServer, "addRecord: %w", err)
  } else {
    return nil
  }
}

func (l *loadTester) rmRecord(rec *pb.Record) *LoadTestErr {
  rec.Tombstone = time.Now().Unix()
  if err := l.srv.IssueRecord(rec); err != nil {
    return derr(ErrServer, "rmRecord: %w")
  }
  return nil
}

func (l *loadTester) requestCompletion(rec *pb.Record) *LoadTestErr {
  g := &pb.Completion{
    Completer: l.srv.ID(),
    Approver: l.srv.ID(),
    Timestamp: time.Now().Unix(),
  }
  if err := l.srv.IssueCompletion(g); err != nil {
    return derr(ErrServer, "requestCompletion: %w", err)
  }
  return nil
}

func (l *loadTester) mutateRecord(rec *pb.Record) *LoadTestErr {
  rec.Location = uuid.New().String()
  if err := l.srv.IssueRecord(rec); err != nil {
    return derr(ErrServer, "mutateRecord: %w", err)
  }
  return nil
}

func (l *loadTester) MakeRandomChange() (string, *LoadTestErr) {
  // Changes possible are 
  // - adding a new record
  // - acquiring (completioning edit) an existing record
  // - removing (tombstoning) an existing record
  // - updating an existing record
  // 
  // Removals and updates require a completion already present; both present and non-present should be tested
  dlog("Making random change")
  /*
  g, err := l.s.GetRandomCompletionWithScope()
  if err != nil {
    dlog("Warning: GetRandomCompletionWithScope: %v", err)
  }
  rec, err := l.s.GetRandomRecord()
  if err != nil {
    dlog("Warning: GetRandomRecord: %v", err)
  }
  nrec, _ := l.s.CountRecords()
  // We become less likely to add a record as we approach the target
  pAdd := float32(l.targetRecords-nrec)/float32(l.targetRecords)
  if rec == nil || rand.Float32() < pAdd {
    return "addRecord()", l.addRecord()
  }

  // Precondition: we have a record and a server, but maybe not a completion.
  if rand.Float32() < 0.9 {
    if g == nil {
      return fmt.Sprintf("requestCompletion(%s, _)", l.srv.ShortID()), l.requestCompletion(rec.Record)
    } else {
      err := l.s.GetSignedRecord(g.Completion.Scope, rec)
      if rec == nil || err != nil {
        return "get record matching completion", derr(ErrNoRecord, "error %v", err)
      }
    }
    // Post: g completions srv access to rec; all entities exist
  }

  if p := rand.Float32(); p < 0.1 {
    return "rmRecord()", l.rmRecord(rec.Record)
  } else if p < 0.3 {
    return "requestCompletion()", l.requestCompletion(rec.Record)
  } else {
    return "mutateRecord()", l.mutateRecord(rec.Record)
  }
  */
  return "", nil
}
