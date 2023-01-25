package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "context"
  "database/sql"
  "os"
  "strings"
  "math"
  "fmt"
  "time"
  //"log"
  _ "github.com/mattn/go-sqlite3"
)

const (
  SchemaPath = "pkg/storage/schema.sql"
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

func (s *sqlite3) createTables(src string) error {
  dat, err := os.ReadFile(src)
  if err != nil {
    return err
  }
  ver := ""
  if err := s.db.QueryRow("SELECT * FROM schemaversion LIMIT 1;").Scan(&ver); err != nil && err != sql.ErrNoRows && err.Error() != "no such table: schemaversion" {
    return fmt.Errorf("check version: %w", err)
  }

  if ver == "" {
    if _, err := s.db.Exec(string(dat)); err != nil {
      return fmt.Errorf("create tables: %w", err)
    }
    if _, err := s.db.Exec(`INSERT INTO schemaversion (version) VALUES ("0.0.1");`); err != nil {
      return fmt.Errorf("write schema version: %w", err)
    }
  } else {
    fmt.Errorf("Schema version %s", ver);
  }
  return nil
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


type scannable interface {
  Scan(...any) error
}

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
  result.Record.Tags = strings.Split(tags, ",")
  return nil
}

func scanSignedCompletion(r scannable, result *pb.SignedCompletion) error {
  // Order must match that of CREATE TABLE statement in createTables()
  return r.Scan(
    &result.Completion.Uuid,
    &result.Completion.Completer,
    &result.Completion.CompleterState,
    &result.Completion.Timestamp,
    &result.Signature.Signer,
    &result.Signature.Data,
  );
}

func (s *sqlite3) GetSignedSourceRecord(uuid string) (*pb.SignedRecord, error) {
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
    INSERT OR REPLACE INTO "completions" (uuid, completer, completer_state, timestamp, signer, signature)
    VALUES (?, ?, ?, ?, ?, ?);
  `, g.Completion.Uuid, g.Completion.Completer, g.Completion.CompleterState, g.Completion.Timestamp, g.Signature.Signer, g.Signature.Data)
  return err
}

func (s *sqlite3) CollapseCompletions(uuid string, signer string) error {
  _, err := s.db.Exec(`
    DELETE FROM "completions" WHERE uuid=? AND signer!=?
  `, uuid, signer)
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

func (s *sqlite3) CountRecordSigners(exclude string) (int64, error) {
  // TODO consider memoization
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(DISTINCT(signer)) FROM "records" WHERE signer != ?;`, exclude).Scan(&num)
  if err == sql.ErrNoRows {
    return 0, nil
  }
  return num, err
}

func (s *sqlite3) CountSignerRecords(signer string) (int64, error) {
  // TODO consider memoization
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(DISTINCT(uuid)) FROM "records" WHERE signer=?;`, signer).Scan(&num)
  if err == sql.ErrNoRows {
    return 0, nil
  }
  return num, err
}

func (s *sqlite3) CountCompletionSigners(exclude string) (int64, error) {
  // TODO consider memoization
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(DISTINCT(signer)) FROM "completions" WHERE signer != ?;`, exclude).Scan(&num)
  if err == sql.ErrNoRows {
    return 0, nil
  }
  return num, err
}

func (s *sqlite3) CountSignerCompletions(peer string) (int64, error) {
  // TODO consider memoization
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(DISTINCT(uuid)) FROM "completions" WHERE signer=?;`, peer).Scan(&num)
  if err == sql.ErrNoRows {
    return 0, nil
  }
  return num, err
}

func(s *sqlite3) SetId(id string) {
	s.id = id
}

func NewSqlite3(path string) (*sqlite3, error) {
  if path != ":memory:" {
    path = "file:" + path + "?_journal_mode=WAL"
  }
  db, err := sql.Open("sqlite3", path)
  if err != nil {
    return nil, fmt.Errorf("failed to open db at %s: %w", path, err)
  }
  db.SetMaxOpenConns(1) // TODO verify needed for non-inmemory
  s := &sqlite3{
    path: path,
    db: db,
		id: "",
    lastCleanup: time.Unix(0,0),
  }
  if err := s.createTables(SchemaPath); err != nil {
    return nil, fmt.Errorf("failed to create tables: %w", err)
  }
  return s, nil
}

func (s *sqlite3) GetSummary() *Summary {
  
  nrec := int64(-1)
  s.db.QueryRow(`SELECT COUNT(*) FROM "records";`).Scan(&nrec)
  ncmp := int64(-1)
  s.db.QueryRow(`SELECT COUNT(*) FROM "completions";`).Scan(&ncmp)
  mtrust := int64(-1)
  s.db.QueryRow(`
		SELECT trust
		FROM "trust"
		ORDER BY trust
		LIMIT 1
		OFFSET (SELECT COUNT(*)
						FROM "trust") / 2
  `).Scan(&mtrust)
  mwrk := int64(-1)
  s.db.QueryRow(`
		SELECT workability
		FROM "workability"
		ORDER BY workability
		LIMIT 1
		OFFSET (SELECT COUNT(*)
						FROM "workability") / 2
  `).Scan(&mwrk)

  return &Summary{
    Location: s.path,
    TotalRecords: nrec,
    TotalCompletions: ncmp,
    LastCleanup: s.lastCleanup.Unix(),
    MedianTrust: mtrust,
    MedianWorkability: mwrk,
    Stats: s.db.Stats(),
  }
}

func (s *sqlite3) SetTrust(peer string, trust float64) error {
  if _, err := s.db.Exec(`
    INSERT OR REPLACE INTO trust (peer, trust, timestamp)
    VALUES (?, ?, ?);
    `, peer, trust, time.Now().Unix()); err != nil {
      return fmt.Errorf("setTrust: %w", err)
  }
  return nil
}

func (s *sqlite3) GetTrust(peer string) (float64, error) {
  // TODO consider memoization
  num := float64(0)
  err := s.db.QueryRow(`SELECT trust FROM "trust" WHERE peer=?;`, peer).Scan(&num)
  if err == sql.ErrNoRows {
    return 0, nil
  }
  return num, err
}

func (s *sqlite3) SetWorkability(uuid string, workability float64) error {
  if _, err := s.db.Exec(`
    INSERT OR REPLACE INTO workability (uuid, workability, timestamp)
    VALUES (?, ?, ?);
    `, uuid, workability, time.Now().Unix()); err != nil {
      return fmt.Errorf("SetWorkability: %w", err)
  }
  return nil
}

func (s *sqlite3) AppendEvent(event string, details string) error {
	if _, err := s.db.Exec(`
		INSERT INTO events (event, details, timestamp)
		VALUES (?, ?, ?);
	`, event, details, time.Now().Unix()); err != nil {
		return fmt.Errorf("AppendEvent: %w", err)
	}
	return nil
}

func (s *sqlite3) GetEvents(ctx context.Context, cur chan<- DBEvent, limit int) error {
  defer close(cur)
  rows, err := s.db.Query("SELECT * FROM events ORDER BY timestamp DESC limit ?;", limit)
  if err != nil {
    return fmt.Errorf("GetEvents SELECT: %w", err)
  }
  defer rows.Close()
  for rows.Next() {
    select {
    case <-ctx.Done():
      return fmt.Errorf("Context canceled")
    default:
    }
		e := DBEvent{}
    if err := rows.Scan(&e.Event, &e.Details, &e.Timestamp); err != nil {
      return fmt.Errorf("GetEvents scan: %w", err)
    }
    cur<- e
  }
  return nil
}
