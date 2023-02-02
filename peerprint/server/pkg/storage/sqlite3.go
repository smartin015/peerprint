package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "context"
  "math/rand"
  "database/sql"
  "strings"
  "fmt"
  "time"
  //"log"
  _ "github.com/mattn/go-sqlite3"
  _ "embed"
)

//go:embed schema.sql
var schema string

type sqlite3 struct {
  path string
  db *sql.DB
  id string
  lastCleanupStart time.Time
	lastCleanupEnd time.Time
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

func (s *sqlite3) genTestTimeline() error {
  start := time.Now().Unix() - (5*60*100)
  for i := 0; i < 100; i++ {
    n :=  rand.Intn(20)
    for j := 0; j < n; j++ {
      if _, err := s.db.Exec(`
        INSERT INTO "census" (peer, timestamp)
        VALUES ($1, $2)
      `, fmt.Sprintf("peer%d", j), start + int64(5*60*i)); err != nil {
        return fmt.Errorf("genTestTimeline: %w", err)
      }
    }
    if _, err := s.db.Exec(`
      INSERT INTO "peers" (peer, first_seen, last_seen)
      VALUES ($1, $2, $3)
    `, fmt.Sprintf("peer%d", i), 1, 2); err != nil {
      return fmt.Errorf("genTestTimeline: %w", err)
    }
  }
  return nil
}

func (s *sqlite3) createTables() error {
  ver := ""
  if err := s.db.QueryRow("SELECT * FROM schemaversion LIMIT 1;").Scan(&ver); err != nil && err != sql.ErrNoRows && err.Error() != "no such table: schemaversion" {
    return fmt.Errorf("check version: %w", err)
  }

  if ver == "" {
    if _, err := s.db.Exec(string(schema)); err != nil {
      return fmt.Errorf("create tables: %w", err)
    }
    if _, err := s.db.Exec(`INSERT INTO schemaversion (version) VALUES ("0.0.1");`); err != nil {
      return fmt.Errorf("write schema version: %w", err)
    }
  } else {
    fmt.Errorf("Schema version %s", ver);
  }

  // return s.genTestTimeline()
  return nil
}

func (s *sqlite3) TrackPeer(signer string) error {
	if _, err := s.db.Exec(`
		INSERT INTO "peers" (peer, first_seen, last_seen)
		VALUES ($1, $2, $2)
		ON CONFLICT(peer) DO UPDATE SET last_seen=$2
	`, signer, time.Now().Unix()); err != nil {
		return fmt.Errorf("set last_seen: %w", err)
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
  if tags != "" {
    result.Record.Tags = strings.Split(tags, ",")
  }
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

func shorten(s string) string {
  if len(s) < 5 {
    return s
  }
  return s[len(s)-4:]
}

func (s *sqlite3) ValidateRecord(r *pb.Record, peer string, maxRecordsPerPeer int64, maxTrackedPeers int64) error {
  // Reject if neither we nor the peer are the approver
  if r.Approver != peer && r.Approver != s.id {
    return fmt.Errorf("Approver=%s, want peer (%s) or self (%s)", shorten(r.Approver), shorten(peer), shorten(s.id))
  }
  // Reject record if peer already has MaxRecordsPerPeer
  if num, err := s.countSignerRecords(peer); err != nil {
    return fmt.Errorf("countSignerRecords: %w", err)
  } else if num > maxRecordsPerPeer {
    return fmt.Errorf("MaxRecordsPerPeer exceeded (%d > %d)", num, maxRecordsPerPeer)
  }
  // Or if peer would put us over MaxTrackedPeers
  if num, err := s.countRecordSigners(peer); err != nil {
    return fmt.Errorf("countRecordSigners: %w", err)
  } else if num > maxTrackedPeers {
    return fmt.Errorf("MaxTrackedPeers exceeded (%d > %d)", num, maxTrackedPeers)
  }

  // Otherwise accept
  return nil
}

func (s *sqlite3) ValidateCompletion(c *pb.Completion, peer string, maxTrackedPeers int64) (*pb.SignedRecord, error) {
  sr, err := s.getSignedSourceRecord(c.Uuid)
  if err != nil {
    if err == sql.ErrNoRows {
      return nil, fmt.Errorf("no record matching completion UUID %s", c.Uuid)
    }
    return nil, fmt.Errorf("getSignedSourceRecord: %w", err)
  }
  // Reject if peer would put us over MaxTrackedPeers
  // We don't have a MaxCompletionsPerPeer as it's only possible to
  // write NUM_RECORDS * NUM_PEERS completions - it's bounded by primary key.
  if num, err := s.countCompletionSigners(peer); err != nil {
    return nil, fmt.Errorf("countCompletionSigners: %w", err)
  } else if num > maxTrackedPeers {
    return nil, fmt.Errorf("MaxTrackedPeers exceeded (%d > %d)", num, maxTrackedPeers)
  }
  return sr, nil
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

func (s *sqlite3) SetSignedCompletion(g *pb.SignedCompletion) error {
  if g.Completion == nil || g.Signature == nil {
    return fmt.Errorf("One or more message fields are nil")
  }
  _, err := s.db.Exec(`
    INSERT OR REPLACE INTO "completions" (uuid, completer, completer_state, timestamp, signer, signature)
    VALUES (?, ?, ?, ?, ?, ?);
  `, g.Completion.Uuid, g.Completion.Completer, g.Completion.CompleterState, g.Completion.Timestamp, g.Signature.Signer, g.Signature.Data)
	if err != nil {
		return fmt.Errorf("insert completion: %w", err)
	}
  return nil
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

func (s *sqlite3) countRecordSigners(exclude string) (int64, error) {
  // TODO consider memoization
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(DISTINCT(signer)) FROM "records" WHERE signer != ?;`, exclude).Scan(&num)
  if err == sql.ErrNoRows {
    return 0, nil
  }
  return num, err
}

func (s *sqlite3) countSignerRecords(signer string) (int64, error) {
  // TODO consider memoization
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(DISTINCT(uuid)) FROM "records" WHERE signer=?;`, signer).Scan(&num)
  if err == sql.ErrNoRows {
    return 0, nil
  }
  return num, err
}

func (s *sqlite3) countCompletionSigners(exclude string) (int64, error) {
  // TODO consider memoization
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(DISTINCT(signer)) FROM "completions" WHERE signer != ?;`, exclude).Scan(&num)
  if err == sql.ErrNoRows {
    return 0, nil
  }
  return num, err
}

func (s *sqlite3) countSignerCompletions(peer string) (int64, error) {
  // TODO consider memoization
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(DISTINCT(uuid)) FROM "completions" WHERE signer=?;`, peer).Scan(&num)
  if err == sql.ErrNoRows {
    return 0, nil
  }
  return num, err
}

func (s *sqlite3) countCompletedRecords() (int64, error) {
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(*) FROM "records" R LEFT JOIN "completions" C ON R.uuid=C.uuid AND R.signer=C.signer WHERE C.timestamp > 0 AND r.signer=R.approver;`).Scan(&num)
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
    lastCleanupStart: time.Unix(0,0),
    lastCleanupEnd: time.Unix(0,0),
  }
  if err := s.createTables(); err != nil {
    return nil, fmt.Errorf("failed to create tables: %w", err)
  }
  return s, nil
}

func (s *sqlite3) medianInt64(tbl, col string) int64 {
  m:= int64(-1)
  s.db.QueryRow(`
		SELECT $2,
		FROM $1
		ORDER BY $2 
		LIMIT 1
		OFFSET (SELECT COUNT(*)
						FROM $1) / 2
  `).Scan(&m)
  return m
}

func (s *sqlite3) GetSummary() (*Summary, []error) {
	ts := []TableStat{}
  errs := []error{}
  for _, tbl := range []string{"records", "completions", "events", "peers", "census"} {
		n:= int64(-1)
    if err := s.db.QueryRow("SELECT COUNT(*) FROM " + tbl + ";").Scan(&n); err != nil {
      errs = append(errs, fmt.Errorf("get count for %s: %w", tbl, err))
    }
		ts = append(ts, TableStat{Name: "total " + tbl, Stat: n})
	}
  numCplt, err := s.countCompletedRecords()
  if err != nil {
    errs = append(errs, fmt.Errorf("get completed records: %w", err))
  } else {
    ts = append(ts, TableStat{Name: "tombstoned records", Stat: numCplt})
  }

  return &Summary{
    Location: s.path,
    TableStats: ts,
    Timing: []TimeProfile{
      TimeProfile{
        Name: "Last cleanup",
        Start: s.lastCleanupStart.Unix(),
        End: s.lastCleanupEnd.Unix(),
      },
    },
    DBStats: s.db.Stats(),
  }, errs
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

func (s *sqlite3) LogPeerCrawl(peer string, ts int64) error {
  // Will abort if already present
  _, err := s.db.Exec(`
    INSERT INTO census (peer, timestamp) VALUES (?, ?)
  `, peer, ts)
  return err
}

func (s *sqlite3) GetPeerTracking(ctx context.Context, cur chan<- *TimeProfile, args ...any) error {
  defer close(cur)
  q := `SELECT * FROM "peers";`
  rows, err := s.db.Query(q)
  if err != nil {
    return fmt.Errorf("GetPeerTracking SELECT: %w", err)
  }
  defer rows.Close()
  for rows.Next() {
    select {
    case <-ctx.Done():
      return fmt.Errorf("Context canceled")
    default:
    }
    d := &TimeProfile{}
    if err := rows.Scan(&d.Name, &d.Start, &d.End); err != nil {
      return fmt.Errorf("GetPeerTracking scan: %w", err)
    }
    cur<- d
  }
  return nil
}
func (s *sqlite3) GetPeerTimeline(ctx context.Context, cur chan<- *DataPoint, args ...any) error {
  defer close(cur)
  q := `SELECT timestamp, COUNT(*) FROM "census" GROUP BY timestamp ORDER BY timestamp ASC LIMIT 10000;`
  rows, err := s.db.Query(q)
  if err != nil {
    return fmt.Errorf("GetPeerTimeline SELECT: %w", err)
  }
  defer rows.Close()
  for rows.Next() {
    select {
    case <-ctx.Done():
      return fmt.Errorf("Context canceled")
    default:
    }
    d := &DataPoint{}
    if err := rows.Scan(&d.Timestamp, &d.Value); err != nil {
      return fmt.Errorf("GetPeerTimeline scan: %w", err)
    }
    cur<- d 
  }
  return nil
}
