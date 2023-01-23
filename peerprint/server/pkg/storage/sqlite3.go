package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "context"
  "database/sql"
  "strings"
  "math"
  "fmt"
  "time"
  //"log"
  _ "github.com/mattn/go-sqlite3"
)

type sqlite3 struct {
  path string
  db *sql.DB
  id string
  lastCleanup time.Time
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

func (s *sqlite3) recomputeAllTrust() error {
  thresh := time.Now().Add(-5*time.Minute).Unix() // TODO const

  if row, err := s.db.Query(`SELECT * FROM "trust";`); err != nil {
    return err
  } else {
    defer row.Close()
    for row.Next() {
      peer := ""
      trust := int64(0)
      ts := int64(0)
      if err := row.Scan(&peer, &trust, &ts); err != nil {
        return err
      }
      if ts >= thresh {
        continue
      }

      if newTrust, err := s.ComputePeerTrust(peer); err != nil {
        return err
      } else if _, err := s.db.Exec(`INSERT OR REPLACE INTO "trust" (peer, trust, ts) VALUES (?, ?, ?);`, peer, newTrust, time.Now().Unix()); err != nil {
        return err
      }
    }
  }
  return nil
}

func (s *sqlite3) removeUntrustedPeers() error {
  // trustThresh := 1.0
  return fmt.Errorf("unimpelmented")
}

func (s *sqlite3) Cleanup() error {
  if err := s.recomputeAllTrust(); err != nil {
    return err
  }
  // TODO remove untrustworthy peers
  // TODO recompute workability
  // TODO remove unworkable records
  s.lastCleanup = time.Now()
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
    peer TEXT NOT NULL PRIMARY KEY,
    trust REAL NOT NULL DEFAULT 0,
    timestamp INT NOT NULL
  );`); err != nil {
    return fmt.Errorf("create trust table: %w", err)
  }

  if _, err := tx.Exec(`
  CREATE TABLE census (
    peer TEXT NOT NULL PRIMARY KEY,
    earliest INT NOT NULL,
    latest INT NOT NULL
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
  if err := s.db.QueryRow(`SELECT COUNT(DISTINCT uuid) 
    FROM "completions" 
    WHERE completer=? AND signer=? AND timestamp!=0;`, peer, s.id).Scan(&completions); err != nil && err != sql.ErrNoRows {
    return 0, fmt.Errorf("Count completions: %w", err)
  }
  incomplete := int64(0)
  if err := s.db.QueryRow(`SELECT COUNT(DISTINCT uuid) 
    FROM "completions" 
    WHERE completer=? AND signer=? AND timestamp=0;`, peer, s.id).Scan(&incomplete); err != nil && err != sql.ErrNoRows {
    return 0, fmt.Errorf("Count incomplete: %w", err)
  }

  hearsay := int64(0)
  if err := s.db.QueryRow(`SELECT COUNT(DISTINCT uuid) FROM "completions" WHERE timestamp!=0 AND completer=? AND signer != ?;`, peer, s.id).Scan(&hearsay); err != nil && err != sql.ErrNoRows {
    return 0, fmt.Errorf("Count hearsay: %w", err)
  }
  max_hearsay := int64(0)
  if err := s.db.QueryRow(`SELECT COUNT(DISTINCT uuid) AS n FROM "completions" WHERE timestamp!=0 AND signer != ? GROUP BY completer ORDER BY n DESC LIMIT 1;`, s.id).Scan(&max_hearsay); err != nil && err != sql.ErrNoRows {
    return 0, fmt.Errorf("Max hearsay: %w", err)
  }
  //print("hearsay ", hearsay, " max ",  max_hearsay, " cplt ", completions, " incomp ", incomplete,"\n")
  return math.Max(float64(completions) + float64(hearsay)/(float64(max_hearsay)+1) - float64(incomplete), 0), nil
}

func (s *sqlite3) ComputeRecordWorkability(r *pb.Record) (float64, error) {
  // Sum the trust for all incomplete assertions by workers
  tww := float64(0)
  if err := s.db.QueryRow(`SELECT COALESCE(SUM(T2.trust), 0) FROM "completions" T1 LEFT JOIN "trust" T2 ON T1.completer=T2.peer WHERE T1.uuid=?`, r.Uuid).Scan(&tww); err != nil {
    return 0, fmt.Errorf("Trust-weighted workers: %w", err)
  }
  //print("tww ", tww, "\n")
  pNotWork := 4/(math.Pow(4, tww+1))
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
    INSERT INTO "completions" (uuid, completer, timestamp, signer, signature)
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
    path: path,
    db: db,
    id: id,
    lastCleanup: time.Unix(0,0),
  }
  if err := s.migrateSchema(); err != nil {
    return nil, fmt.Errorf("failed to migrate schema: %w", err)
  }
  return s, nil
}

func (s *sqlite3) GetSummary() *Summary {
  return &Summary{
    Location: s.path,
    TotalRecords: 1,
    TotalCompletions: 2,
    LastCleanup: s.lastCleanup.Unix(),
    MedianTrust: 4,
    MedianWorkability: 5,
    Stats: s.db.Stats(),
  }
}
