package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
	"github.com/libp2p/go-libp2p/core/crypto"
  "context"
  "database/sql"
  "strings"
  "fmt"
  "log"
  _ "github.com/mattn/go-sqlite3"
)

type sqlite3 struct {
  db *sql.DB
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
  CREATE TABLE recordtags (
    name TEXT NOT NULL,
    record TEXT NOT NULL,
    signer TEXT NOT NULL,
    PRIMARY KEY (name, record, signer)
  );`); err != nil {
    return fmt.Errorf("create recordtags table: %w", err)
  }

  if _, err := tx.Exec(`
  CREATE TABLE records (
    uuid TEXT NOT NULL,
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
  CREATE TABLE peers (
    peer TEXT NOT NULL PRIMARY KEY,
    pubkey BLOB NOT NULL,
    trust REAL NOT NULL DEFAULT 0
  );`); err != nil {
    return fmt.Errorf("create pubkeys table: %w", err)
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

func (s *sqlite3) GetPeerTrust(peer string) (float64, error) {
  return 0, fmt.Errorf("Unimplemented")
}

func (s *sqlite3) SetSignedRecord(r *pb.SignedRecord) error {
  if r.Record == nil || r.Signature == nil || r.Record.Rank == nil {
    return fmt.Errorf("One or more message fields are nil")
  }

  tx, err := s.db.Begin()
  if err != nil {
    return err
  }
  defer tx.Rollback()

  // Wipe out and repopulate tags
  if _, err := tx.Exec(`DELETE FROM recordtags WHERE record=$1;`, r.Record.Uuid); err != nil {
    return fmt.Errorf("delete tags: %w", err)
  }

  stmt, err := tx.Prepare(`INSERT INTO recordtags (name, record, signer) VALUES (?, ?, ?)`)
  if err != nil {
    return fmt.Errorf("prepare tags: %w", err)
  }
  defer stmt.Close()
  for _, tag := range r.Record.Tags {
    if _, err := stmt.Exec(tag, r.Record.Uuid, r.Signature.Signer); err != nil {
      return fmt.Errorf("insert tag %s: %w", tag, err)
    }
  }

  if _, err = tx.Exec(`
    INSERT INTO records (uuid, approver, location, created, tombstone, worker, worker_state, num, den, gen, signer, signature)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
  `,
    r.Record.Uuid,
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
  return tx.Commit()
}


type scannable interface {
  Scan(...any) error
}

func scanSignedRecord(r scannable, result *pb.SignedRecord) error {
  // Order must match that of CREATE TABLE statement in migrateSchema()
  return r.Scan(
    &result.Record.Uuid,
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
  );
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

func (s *sqlite3) GetSignedRecord(uuid string, result *pb.SignedRecord) error {
  clearSignedRecord(result)
  if err := scanSignedRecord(s.db.QueryRow(`SELECT * FROM "records" WHERE uuid=$1 AND tombstone=0 LIMIT 1;`, uuid), result); err != nil {
    if err == sql.ErrNoRows {
      return err // Allow for comparison upstream
    }
    return fmt.Errorf("select record: %w", err)
  }

  // Import tags now that we have a signature
  rows, err := s.db.Query(`SELECT name FROM recordtags WHERE record=$1 AND signer=$2`, uuid, result.Signature.Signer)
  if err != nil {
    return fmt.Errorf("fetch tags: %w", err)
  } else {
    tag := ""
    for rows.Next() {
      if err := rows.Scan(&tag); err != nil {
        rows.Close()
        return fmt.Errorf("fetch single tag: %w", err)
      }
      result.Record.Tags = append(result.Record.Tags, tag)
    }
    rows.Close()
  }

  return nil
}

func (s *sqlite3) GetSignedRecords(ctx context.Context, cur chan<- *pb.SignedRecord, opts ...any) error {
  where := []string{}
  args := []any{}
  for _, opt := range opts {
    switch v := opt.(type) {
    case WithUUID:
      where = append(where,  "uuid=?")
      args = append(args, string(v))
    default:
        return fmt.Errorf("GetSignedRecords received invalid option: %v", opt)
    }
  }
  q := `SELECT * FROM "records" WHERE tombstone=0 `
  if len(where) > 0 {
    q += strings.Join(where, " AND ")
  }
  q += "GROUP BY uuid;"
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
    log.Println("Scanning record")
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
  where := []string{}
  args := []any{}
  for _, opt := range opts {
    switch v := opt.(type) {
    case WithSigner:
      where = append(where,  "signer=?")
      args = append(args, string(v))
    case WithUUID:
      where = append(where, "uuid=?")
      args = append(args, string(v))
    default:
        return fmt.Errorf("GetSignedCompletions received invalid option: %v", opt)
    }
  }
  q := `SELECT * FROM "completions"`
  if len(where) > 0 {
    q += " WHERE " + strings.Join(where, " AND ")
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

func (s *sqlite3) SetPubKey(peer string, pubkey crypto.PubKey) error {
  b, err := crypto.MarshalPublicKey(pubkey)
  if err != nil {
    return fmt.Errorf("Unwrap key: %w", err)
  }
  if _, err = s.db.Exec(`
    INSERT OR REPLACE INTO "peers" (peer, pubkey)
    VALUES (?, ?);
  `, peer, b); err != nil {
    return fmt.Errorf("SetPubKey: %w", err)
  }
  return nil
}

func (s *sqlite3) GetPubKey(peer string) (crypto.PubKey, error) {
  var b []byte
  if err := s.db.QueryRow(`SELECT pubkey FROM "peers" WHERE peer=? LIMIT 1;`, peer).Scan(&b); err != nil {
    return nil, err
  }
  return crypto.UnmarshalPublicKey(b)
}

func (s *sqlite3) CountRecordUUIDs() (int64, error) {
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(*) FROM "records" WHERE tombstone=0 GROUP BY uuid;`).Scan(&num)
  return num, err
}

func NewSqlite3(path string) (*sqlite3, error) {
  db, err := sql.Open("sqlite3", path)
  if err != nil {
    return nil, fmt.Errorf("failed to open db at %s: %w", path, err)
  }
  db.SetMaxOpenConns(1) // TODO verify needed for non-inmemory
  s := &sqlite3{
    db: db,
  }
  if err := s.migrateSchema(); err != nil {
    return nil, fmt.Errorf("failed to migrate schema: %w", err)
  }
  return s, nil
}
