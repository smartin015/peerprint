package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "database/sql"
  "context"
)

var (
  ErrNoRows = sql.ErrNoRows
)

type WithSigner string
type WithLimit int

type Interface interface {
  SetSignedRecord(r *pb.SignedRecord) error
  GetSignedRecords(context.Context, chan<- *pb.SignedRecord, ...any) error

  SetSignedCompletion(g *pb.SignedCompletion) error
  GetSignedCompletions(context.Context, chan<- *pb.SignedCompletion, ...any) error

  ComputePeerTrust(peer string) (float64, error)
  ComputeRecordWorkability(r *pb.Record) (float64, error)
  Cleanup() error

  GetSummary() *Summary
}

type Summary struct {
  Location string
  TotalRecords int64
  TotalCompletions int64
  LastCleanup int64
  MedianTrust int64
  MedianWorkability int64
  Stats sql.DBStats
}
