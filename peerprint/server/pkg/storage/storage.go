package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
	"github.com/libp2p/go-libp2p/core/crypto"
  "database/sql"
)

var (
  ErrNoRows = sql.ErrNoRows
)

type WithType pb.GrantType
type WithTarget string
type WithPeer string
type WithScope string
type WithUUID string
type IncludeExpired bool

type Interface interface {
  // StreamRecords(tags []string, results chan<- *pb.Record) error

  SetSignedRecord(r *pb.SignedRecord) error
  GetSignedRecord(uuid string, result *pb.SignedRecord) error
  GetSignedRecords(opts ...any) ([]*pb.SignedRecord, error)
  GetRandomRecord() (*pb.SignedRecord, error)
  CountRecords() (int64, error)

  SetSignedGrant(g *pb.SignedGrant) error
  GetSignedGrants(opts ...any) ([]*pb.SignedGrant, error)
  GetRandomGrantWithScope() (*pb.SignedGrant, error)

  SetPubKey(peer string, pubkey crypto.PubKey) error
  GetPubKey(peer string) (crypto.PubKey, error)

  IsAdmin(peer string) (bool, error)
  CountAdmins() (int64, error)
  // ValidGrants returns true if there are valid grants stored.
  ValidGrants() (bool, error)
  // CanEdit(peer string, uuid string) (bool, error)
}

