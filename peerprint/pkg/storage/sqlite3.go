package storage

import (
  "database/sql"
  "fmt"
  "time"
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

func (s *sqlite3) Close() {
  s.db.Close()
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

func shorten(s string) string {
  if len(s) < 5 {
    return s
  }
  return s[len(s)-4:]
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
    return nil, fmt.Errorf("failed to create tables (%s): %w", path, err)
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
  for _, tbl := range []string{"records", "completions", "events", "peers", "census", "clients"} {
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

