package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "database/sql"
  "context"
  "fmt"
)

var (
  ErrNoRows = sql.ErrNoRows
  handler Interface
)

type WithSigner string
type WithLimit int

type DBEvent struct {
  Event string
  Details string
  Timestamp int64
}

type Interface interface {
  SetId(id string)

  ValidateRecord(r *pb.Record, peer string, maxRecordsPerPeer int64, maxTrackedPeers int64) error
  SetSignedRecord(*pb.SignedRecord) error
  GetSignedRecords(context.Context, chan<- *pb.SignedRecord, ...any) error

  ValidateCompletion(c *pb.Completion, peer string, maxTrackedPeers int64) (*pb.SignedRecord, error)
  SetSignedCompletion(*pb.SignedCompletion) error
  GetSignedCompletions(context.Context, chan<- *pb.SignedCompletion, ...any) error
  CollapseCompletions(uuid string, signer string) error

  Cleanup(until_records int64) (int64, error)
  GetSummary() (*Summary, []error)

  AppendEvent(event string, details string) error
  GetEvents(ctx context.Context, cur chan<- DBEvent, limit int) error

	TrackPeer(signer string) error
  LogPeerCrawl(peer string, ts int64) error
}

func SetPanicHandler(s Interface) {
  handler = s
}

func  HandlePanic() {
  if pnk := recover(); pnk != nil {
    handler.AppendEvent("panic", fmt.Sprintf("%v", pnk)) // Ignore error; best effort
    panic(pnk)
  }
}

type TableStat struct {
	Name string
	Stat int64
}

type TimeProfile struct {
  Name string
  Start int64
  End int64
}

type Summary struct {
  Location string
  Timing []TimeProfile
  TableStats []TableStat
  DBStats sql.DBStats
}
