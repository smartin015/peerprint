package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "database/sql"
  "context"
)

var (
  ErrNoRows = sql.ErrNoRows
  handler Interface
)

type WithSigner string
type WithUUID string
type WithLimit int

type DataPoint struct {
	Timestamp int64
	Value int64
}

type DBEvent struct {
  Event string
  Details string
  Timestamp int64
}

type Interface interface {
  Close()
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
  SetPeerStatus(peer string, status *pb.PeerStatus) error
  GetPrinterLocations(context.Context, int64, chan<- *pb.Location) error
  LogPeerCrawl(peer string, ts int64) error
  GetPeerTracking(context.Context, chan<- *TimeProfile, ...any) error
  GetPeerTimeline(context.Context, chan<- *DataPoint, ...any) error
}

type Registry interface {
  UpsertConfig(*pb.NetworkConfig, []byte, string) error
  DeleteConfig(uuid string, tbl string) error
  SignConfig(uuid string, sig []byte) error
  UpsertStats(uuid string, stats *pb.NetworkStats) error
  GetRegistry(context.Context, chan<- *pb.Network, string,bool) error
}

func SetPanicHandler(s Interface) {
  handler = s
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
