package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
	"github.com/libp2p/go-libp2p/core/crypto"
)

type WithTarget string
type WithPeer string
type WithScope string
type IncludeExpired bool

type Interface interface {
  // StreamRecords(tags []string, results chan<- *pb.Record) error

  SetSignedRecord(r *pb.SignedRecord) error
  GetSignedRecord(uuid string, result *pb.SignedRecord) error
  GetSignedRecords() ([]*pb.SignedRecord, error)

  SetSignedGrant(g *pb.SignedGrant) error
  GetSignedGrants(opts ...any) ([]*pb.SignedGrant, error)

  SetPubKey(peer string, pubkey crypto.PubKey) error
  GetPubKey(peer string) (crypto.PubKey, error)

  IsAdmin(peer string) (bool, error)
  CountAdmins() (int, error)
  // ValidGrants returns true if there are valid grants stored.
  ValidGrants() (bool, error)
  // CanEdit(peer string, uuid string) (bool, error)
}

