package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "context"
  "database/sql"
  "strings"
  "fmt"
  _ "github.com/mattn/go-sqlite3"
  _ "embed"
)

func scanSignedRecord(r scannable, result *pb.SignedRecord) error {
  // Order must match that of CREATE TABLE statement in createTables()
  tags := ""
  if err := r.Scan(
    &result.Record.Uuid,
    &tags,
    &result.Record.Approver,
    &result.Record.Manifest,
    &result.Record.Created,
    &result.Record.Rank.Num,
    &result.Record.Rank.Den,
    &result.Record.Rank.Gen,
    &result.Signature.Signer,
    &result.Signature.Data,
  ); err != nil {
    return err
  }
  if tags != "" {
    result.Record.Tags = strings.Split(tags, ",")
  }
  return nil
}


func clearSignedRecord(rec *pb.SignedRecord) {
  // Init with empty objects to prevent segfault
  rec.Signature= &pb.Signature{}
  rec.Record= &pb.Record{
    Rank: &pb.Rank{},
    Tags: []string{},
  }
}

func (s *sqlite3) countSignerRecords(signer string) (int64, error) {
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(DISTINCT(uuid)) FROM "records" WHERE signer=?;`, signer).Scan(&num)
  if err == sql.ErrNoRows {
    return 0, nil
  }
  return num, err
}

func (s *sqlite3) countRecordSigners(exclude string) (int64, error) {
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(DISTINCT(signer)) FROM "records" WHERE signer != ?;`, exclude).Scan(&num)
  if err == sql.ErrNoRows {
    return 0, nil
  }
  return num, err
}

func (s *sqlite3) ValidateRecord(r *pb.Record, peer string, maxRecordsPerPeer int64, maxTrackedPeers int64) error {
  // Reject if neither we nor the peer are the approver
  if r.Approver != peer && r.Approver != s.id {
    return fmt.Errorf("Approver=%s, want peer (%s) or self (%s)", shorten(r.Approver), shorten(peer), shorten(s.id))
  }
  // Reject record if peer already has MaxRecordsPerPeer
  if num, err := s.countSignerRecords(peer); err != nil {
    return fmt.Errorf("countSignerRecords: %w", err)
  } else if num+1 > maxRecordsPerPeer {
    return fmt.Errorf("MaxRecordsPerPeer exceeded (%d > %d)", num, maxRecordsPerPeer)
  }
  // Or if peer would put us over MaxTrackedPeers
  if num, err := s.countRecordSigners(peer); err != nil {
    return fmt.Errorf("countRecordSigners: %w", err)
  } else if num+1 > maxTrackedPeers {
    return fmt.Errorf("MaxTrackedPeers exceeded (%d > %d)", num, maxTrackedPeers)
  }

  // Otherwise accept
  return nil
}

func (s *sqlite3) SetSignedRecord(r *pb.SignedRecord) error {
  if r.Record == nil || r.Signature == nil || r.Record.Rank == nil {
    return fmt.Errorf("One or more message fields (Record, Signature, or Rank) are nil")
  }

  if _, err := s.db.Exec(`
    INSERT OR REPLACE INTO records (uuid, tags, approver, manifest, created, num, den, gen, signer, signature)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
  `,
    r.Record.Uuid,
    strings.Join(r.Record.Tags, ","),
    r.Record.Approver,
    r.Record.Manifest,
    r.Record.Created,
    r.Record.Rank.Num,
    r.Record.Rank.Den,
    r.Record.Rank.Gen,
    r.Signature.Signer,
    r.Signature.Data); err != nil {
    return fmt.Errorf("insert into records: %w", err)
  }
  return nil
}

func (s *sqlite3) getSignedSourceRecord(uuid string) (*pb.SignedRecord, error) {
  result := &pb.SignedRecord{}
  clearSignedRecord(result)
  if err := scanSignedRecord(s.db.QueryRow(`SELECT * FROM "records" WHERE uuid=? AND approver=signer LIMIT 1;`, uuid), result); err != nil {
    return nil, err
  }
  return result, nil
}

func (s *sqlite3) GetSignedRecords(ctx context.Context, cur chan<- *pb.SignedRecord, opts ...any) error {
  defer close(cur)
  where := []string{}
  args := []any{}
  limit := -1
  for _, opt := range opts {
    switch v := opt.(type) {
    case WithSigner:
      where = append(where,  "signer=?")
      args = append(args, string(v))
    case WithUUID:
      where = append(where,  "uuid=?")
      args = append(args, string(v))
    case WithLimit:
      limit = int(v)
    default:
        return fmt.Errorf("GetSignedRecords received invalid option: %v", opt)
    }
  }
  q := `SELECT * FROM "records" `
  if len(where) > 0 {
    q += "WHERE " + strings.Join(where, " AND ")
  }
  if limit > 0 {
    q += fmt.Sprintf(" LIMIT %d", limit)
  }
  q += ";"
  rows, err := s.db.Query(q, args...)
  if err != nil {
    return fmt.Errorf("GetSignedRecords SELECT: %w", err)
  }
  defer rows.Close()

  for rows.Next() {
    select {
    case <-ctx.Done():
      return fmt.Errorf("Context canceled")
    default:
    }

    sr := &pb.SignedRecord{
      Signature: &pb.Signature{},
      Record: &pb.Record{Rank: &pb.Rank{}},
    }
    if err := scanSignedRecord(rows, sr); err != nil {
      return fmt.Errorf("GetSignedRecords scan: %w", err)
    }
    cur<- sr
  }
  return nil
}

