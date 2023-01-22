package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
	"github.com/libp2p/go-libp2p/core/crypto"
  "database/sql"
  "context"
)

var (
  ErrNoRows = sql.ErrNoRows
)

type WithSigner string
type WithUUID string
type WithRandomized bool
type WithLimit int

type Interface interface {
  SetSignedRecord(r *pb.SignedRecord) error

  GetSignedRecord(uuid string, result *pb.SignedRecord) error
  GetSignedRecords(context.Context, chan<- *pb.SignedRecord, ...any) error

  SetSignedCompletion(g *pb.SignedCompletion) error
  GetSignedCompletions(context.Context, chan<- *pb.SignedCompletion, ...any) error

  SetPubKey(peer string, pubkey crypto.PubKey) error
  GetPubKey(peer string) (crypto.PubKey, error)

  GetPeerTrust(peer string) (float64, error)

  Cleanup() error
}

