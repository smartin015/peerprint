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

  SetSignedRecord(r *pb.SignedRecord) error
  GetSignedRecords(context.Context, chan<- *pb.SignedRecord, ...any) error
  GetSignedSourceRecord(uuid string) (*pb.SignedRecord, error)
  CountRecordSigners(exclude string) (int64, error)
  CountSignerRecords(signer string) (int64, error)

  SetSignedCompletion(g *pb.SignedCompletion) error
  GetSignedCompletions(context.Context, chan<- *pb.SignedCompletion, ...any) error
  CountCompletionSigners(exclude string) (int64, error)
  CountSignerCompletions(signer string) (int64, error)
  CollapseCompletions(uuid string, signer string) error

  Cleanup(untilPeers int64) []error
  GetSummary() *Summary

  GetWorkerTrust(peer string) (float64, error)
  // GetRewardTrust not provided - we should never need to get the reward trust, as we delegate work-picking to the (python) wrapper 
  SetWorkerTrust(peer string, trust float64) error
  SetRewardTrust(peer string, trust float64) error

  SetWorkability(uuid string, origin string, workability float64) error

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
