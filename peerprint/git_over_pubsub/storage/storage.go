package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/proto"
	"github.com/libp2p/go-libp2p/core/crypto"
  "fmt"
)

type Interface interface {
  // StreamRecords(tags []string, results chan<- *pb.Record) error

  SetSignedRecord(r *pb.SignedRecord) error
  GetSignedRecord(uuid string, result *pb.SignedRecord) error
  GetSignedRecords() ([]*pb.SignedRecord, error)

  SetSignedGrant(g *pb.SignedGrant) error
  GetSignedGrants() ([]*pb.SignedGrant, error)

  SetPubKey(peer string, pubkey crypto.PubKey) error
  GetPubKey(peer string) (crypto.PubKey, error)

  IsAdmin(peer string) (bool, error)
  CountAdmins() (int, error)
  // ValidGrants returns true if there are valid grants stored.
  ValidGrants() (bool, error)
  // CanEdit(peer string, uuid string) (bool, error)
}

type inmem struct {
  records map[string]*pb.SignedRecord
  grants map[string][]*pb.SignedGrant
  keys map[string]crypto.PubKey
}

func NewInMemory() Interface {
  return &inmem {
    records: make(map[string]*pb.SignedRecord),
    grants: make(map[string][]*pb.SignedGrant),
    keys: make(map[string]crypto.PubKey),
  }
}

func (s *inmem) SetPubKey(peer string, pubkey crypto.PubKey) error {
  s.keys[peer] = pubkey
  return nil
}
func (s *inmem) GetPubKey(peer string) (crypto.PubKey, error) {
  if k, ok := s.keys[peer]; !ok {
    return nil, fmt.Errorf("No pubkey for peer %s", peer)
  } else {
    return k, nil
  }
}

func (s *inmem) GetSignedRecord(uuid string, result *pb.SignedRecord) error {
  if r, ok := s.records[uuid]; !ok {
    return fmt.Errorf("Record %s not found", uuid)
  } else {
    // TODO clone
    result = r
    return nil
  }
}

func (s *inmem) GetSignedRecords() ([]*pb.SignedRecord, error) {
  r := []*pb.SignedRecord{}
  for _, rec := range s.records {
    r = append(r, rec)
  }
  return r, nil
}

func (s *inmem)  SetSignedRecord(r *pb.SignedRecord) error {
  if rec, ok := s.records[r.Record.Uuid]; ok {
    if rec.Record.Version > r.Record.Version {
      return fmt.Errorf("Existing record with version %d (above candidate %d)", rec.Record.Version, r.Record.Version)
    }
  }
  s.records[r.Record.Uuid] = r
  return nil
}

func (s *inmem)  SetSignedGrant(g *pb.SignedGrant) error {
  if gg, ok := s.grants[g.Grant.Target]; !ok {
    s.grants[g.Grant.Target] = []*pb.SignedGrant{g}
  } else {
    for i, sg := range gg {
      if sg.Grant.Type == g.Grant.Type {
        s.grants[g.Grant.Target][i] = g
        return nil
      }
    }
    s.grants[g.Grant.Target] = append(gg, g)
  }
  return nil
}

func (s *inmem) GetSignedGrants() ([]*pb.SignedGrant, error) {
  r := []*pb.SignedGrant{}
  for _, pg := range s.grants {
    for _, sg := range pg {
      // TODO Clone
      r = append(r, sg)
    }
  }
  return r, nil
}

func (s *inmem) ValidGrants() (bool, error) {
  return len(s.grants) > 0, nil
}

func (s *inmem) IsAdmin(peer string) (bool, error) {
  // When there are no grants, accept any grant (bootstrap)
  if len(s.grants[peer]) == 0 {
    return true, nil 
  }
  if gg, ok := s.grants[peer]; ok {
    for _, g := range gg {
      if g.Grant.Type == pb.GrantType_ADMIN {
        return true, nil
      }
    }
  }
  return false, nil
}

func (s *inmem) CountAdmins() (int, error) {
  c := make(map[string]struct{})
  gg, err := s.GetSignedGrants()
  if err != nil {
    return 0, err
  }
  for _, g := range gg {
    if g.Grant.Type == pb.GrantType_ADMIN {
      c[g.Grant.Target] = struct{}{}
    }
  }
  return len(c), nil
}
