package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "context"
  "database/sql"
  "strings"
  "math"
  "fmt"
  //"log"
  _ "github.com/mattn/go-sqlite3"
)

type sqlite3 struct {
  db *sql.DB
  id string
}

func clearSignedRecord(rec *pb.SignedRecord) {
  // Init with empty objects to prevent segfault
  rec.Signature= &pb.Signature{}
  rec.Record= &pb.Record{
    Rank: &pb.Rank{},
    Tags: []string{},
  }
}

func (s *sqlite3) Close() {
  s.db.Close()
}

func (s *sqlite3) Cleanup() error {
  return fmt.Errorf("Not implemented")
}

func (s *sqlite3) migrateSchema() error {
  tx, err := s.db.Begin()
  if err != nil {
    return err
  }
  defer tx.Rollback()

  if _, err := tx.Exec(`
  CREATE TABLE records (
    uuid TEXT NOT NULL,
    tags TEXT NOT NULL,
    approver TEXT NOT NULL,
    location TEXT NOT NULL,
    created INT NOT NULL,
    tombstone INT,
  
    num INT NOT NULL,
    den INT NOT NULL,
    gen REAL NOT NULL,

    worker TEXT,
    worker_state TEXT,

    signer BLOB NOT NULL,
    signature BLOB NOT NULL
  );`); err != nil {
    return fmt.Errorf("create records table: %w", err)
  }

  if _, err := tx.Exec(`
  CREATE TABLE workability (
    uuid TEXT NOT NULL PRIMARY KEY,
    timestamp INT NOT NULL,
    workability REAL NOT NULL
  );`); err != nil {
      return fmt.Errorf("create workability table: %w", err)
  }

  if _, err := tx.Exec(`
  CREATE TABLE trust (
    id TEXT NOT NULL PRIMARY KEY,
    trust REAL NOT NULL DEFAULT 0,
    timestamp INT NOT NULL
  );`); err != nil {
    return fmt.Errorf("create trust table: %w", err)
  }

  if _, err := tx.Exec(`
  CREATE TABLE census (
    peer TEXT NOT NULL PRIMARY KEY,
    timestamp INT NOT NULL
  );`); err != nil {
    return fmt.Errorf("create census table: %w", err)
  }

  if _, err := tx.Exec(`
  CREATE TABLE completions (
    uuid TEXT NOT NULL,
    completer TEXT NOT NULL,
    timestamp INT NOT NULL,

    signer TEXT NOT NULL,
    signature BLOB NOT NULL,
    PRIMARY KEY (uuid, signer)
  );`); err != nil {
    return fmt.Errorf("create completions table: %w", err)
  }

  return tx.Commit()
}

func (s *sqlite3) ComputePeerTrust(peer string) (float64, error) {
  completions := int64(0)
  incomplete := int64(0)
  if err := s.db.QueryRow(`SELECT COUNTIF(timestamp IS NULL), COUNTIF(timestamp IS NOT NULL) FROM "completions" WHERE completer=? AND approver=? GROUP BY uuid;`, peer, s.id).Scan(&incomplete, &completions); err != nil {
    return 0, fmt.Errorf("Count completions: %w", err)
  }
  hearsay := int64(0)
  if err := s.db.QueryRow(`SELECT COUNT(*) FROM "completions" WHERE timestamp IS NOT NULL AND completer=? AND approver != ? GROUP BY uuid;`, peer, s.id).Scan(&hearsay); err != nil {
    return 0, fmt.Errorf("Count hearsay: %w", err)
  }
  max_hearsay := int64(0)
  if err := s.db.QueryRow(`SELECT COUNT(*) AS n FROM "completions" WHERE timestamp IS NOT NULL AND approver != ? GROUP BY uuid, completer ORDER BY n DESC LIMIT 1;`, s.id).Scan(&max_hearsay); err != nil {
    return 0, fmt.Errorf("Max hearsay: %w", err)
  }
  return math.Max(float64(completions) + float64(hearsay)/(float64(max_hearsay)+1) - float64(incomplete), 0), nil
}

func (s *sqlite3) ComputeRecordWorkability(r *pb.Record) (float64, error) {
  tww := float64(0)
  if err := s.db.QueryRow(`SELECT SUM(T2.trust) FROM "completions" T1 LEFT JOIN "peers" T2 ON T1.completer=T2.id WHERE T1.uuid=?`, r.Uuid).Scan(&tww); err != nil {
    return 0, fmt.Errorf("Trust-weighted workers: %w", err)
  }
  pNotWork := 1/(math.Pow(4, tww+1))
  return pNotWork, nil
}

func (s *sqlite3) SetSignedRecord(r *pb.SignedRecord) error {
  if r.Record == nil || r.Signature == nil || r.Record.Rank == nil {
    return fmt.Errorf("One or more message fields are nil")
  }
  if _, err := s.db.Exec(`
    INSERT INTO records (uuid, tags, approver, location, created, tombstone, worker, worker_state, num, den, gen, signer, signature)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
  `,
    r.Record.Uuid,
    strings.Join(r.Record.Tags, ","),
    r.Record.Approver,
    r.Record.Location,
    r.Record.Created,
    r.Record.Tombstone,
    r.Record.Worker,
    r.Record.WorkerState,
    r.Record.Rank.Num,
    r.Record.Rank.Den,
    r.Record.Rank.Gen,
    r.Signature.Signer,
    r.Signature.Data); err != nil {
    return fmt.Errorf("insert into records: %w", err)
  }
  return nil
}


type scannable interface {
  Scan(...any) error
}

func scanSignedRecord(r scannable, result *pb.SignedRecord) error {
  // Order must match that of CREATE TABLE statement in migrateSchema()
  tags := ""
  if err := r.Scan(
    &result.Record.Uuid,
    &tags,
    &result.Record.Approver,
    &result.Record.Location,
    &result.Record.Created,
    &result.Record.Tombstone,
    &result.Record.Rank.Num,
    &result.Record.Rank.Den,
    &result.Record.Rank.Gen,
    &result.Record.Worker,
    &result.Record.WorkerState,
    &result.Signature.Signer,
    &result.Signature.Data,
  ); err != nil {
    return err
  }
  result.Record.Tags = strings.Split(tags, ",")
  return nil
}

func scanSignedCompletion(r scannable, result *pb.SignedCompletion) error {
  // Order must match that of CREATE TABLE statement in migrateSchema()
  return r.Scan(
    &result.Completion.Uuid,
    &result.Completion.Completer,
    &result.Completion.Timestamp,
    &result.Signature.Signer,
    &result.Signature.Data,
  );
}

func (s *sqlite3) GetRandomRecord() (*pb.SignedRecord, error) {
  result := &pb.SignedRecord{}
  clearSignedRecord(result)
  if err := scanSignedRecord(s.db.QueryRow(`SELECT * FROM "records" WHERE tombstone=0 ORDER BY RANDOM() LIMIT 1;`), result); err != nil {
    if err == sql.ErrNoRows {
      return nil, err // Allow for comparison upstream
    }
    return nil, fmt.Errorf("get random record: %w", err)
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
    case WithLimit:
      limit = int(v)
    default:
        return fmt.Errorf("GetSignedRecords received invalid option: %v", opt)
    }
  }
  // TODO join tags
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

func (s *sqlite3) SetSignedCompletion(g *pb.SignedCompletion) error {
  if g.Completion == nil || g.Signature == nil {
    return fmt.Errorf("One or more message fields are nil")
  }
  _, err := s.db.Exec(`
    INSERT OR REPLACE INTO "completions" (uuid, completer, timestamp, signer, signature)
    VALUES (?, ?, ?, ?, ?);
  `, g.Completion.Uuid, g.Completion.Completer, g.Completion.Timestamp, g.Signature.Signer, g.Signature.Data)
  return err
}

func (s *sqlite3) GetSignedCompletions(ctx context.Context, cur chan<- *pb.SignedCompletion, opts ...any) error {
  defer close(cur)
  where := []string{}
  args := []any{}
  limit := -1
  for _, opt := range opts {
    switch v := opt.(type) {
    case WithSigner:
      where = append(where,  "signer=?")
      args = append(args, string(v))
    case WithLimit:
      limit = int(v)
    default:
        return fmt.Errorf("GetSignedCompletions received invalid option: %v", opt)
    }
  }
  q := `SELECT * FROM "completions"`
  if len(where) > 0 {
    q += " WHERE " + strings.Join(where, " AND ")
  }
  if limit > 0 {
    q += fmt.Sprintf(" LIMIT %d", limit)
  }
  q += ";"
  rows, err := s.db.Query(q, args...)
  if err != nil {
    return fmt.Errorf("%s: %w", q, err)
  }
  defer rows.Close()
  for rows.Next() {
    select {
    case <-ctx.Done():
      return fmt.Errorf("Context canceled")
    default:
    }
    sg := &pb.SignedCompletion{Completion: &pb.Completion{}, Signature: &pb.Signature{}}
    if err := scanSignedCompletion(rows, sg); err != nil {
      return fmt.Errorf("GetSignedCompletions scan: %w", err)
    }
    cur<- sg
  }
  return nil
}

func (s *sqlite3) CountRecordUUIDs() (int64, error) {
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(*) FROM "records" WHERE tombstone=0 GROUP BY uuid;`).Scan(&num)
  return num, err
}

func NewSqlite3(path string, id string) (*sqlite3, error) {
  db, err := sql.Open("sqlite3", path)
  if err != nil {
    return nil, fmt.Errorf("failed to open db at %s: %w", path, err)
  }
  db.SetMaxOpenConns(1) // TODO verify needed for non-inmemory
  s := &sqlite3{
    db: db,
    id: id,
  }
  if err := s.migrateSchema(); err != nil {
    return nil, fmt.Errorf("failed to migrate schema: %w", err)
  }
  return s, nil
}
