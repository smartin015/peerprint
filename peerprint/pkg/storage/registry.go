// Functions for managing a network registry
package storage

import (
  "fmt"
  "strings"
  "log"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "database/sql"
  "context"
  _ "github.com/mattn/go-sqlite3"
  _ "embed"
)

const (
  RegistryTable = "registry"
  LobbyTable = "lobby"
)

//go:embed registry_schema.sql
var registrySchema string

type registry struct {
  path string
  db *sql.DB
}

func NewRegistry(path string) (*registry, error) {
  if path != ":memory:" {
    path = "file:" + path + "?_journal_mode=WAL"
  }
  log.Println("NewRegistry: " + path)
  db, err := sql.Open("sqlite3", path)
  if err != nil {
    return nil, fmt.Errorf("failed to open db at %s: %w", path, err)
  }
  db.SetMaxOpenConns(1) // TODO verify needed for non-inmemory
  s := &registry{
    path: path,
    db: db,
  }
  if err := s.createRegistryTables(); err != nil {
    return nil, fmt.Errorf("failed to create tables: %w", err)
  }
  return s, nil
}


func (s *registry) createRegistryTables() error {
  ver := ""
  if err := s.db.QueryRow("SELECT * FROM schemaversion LIMIT 1;").Scan(&ver); err != nil && err != sql.ErrNoRows && err.Error() != "no such table: schemaversion" {
    return fmt.Errorf("check version: %w", err)
  }

  if ver == "" {
    if _, err := s.db.Exec(string(registrySchema)); err != nil {
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

func (s *registry) Close() {
  s.db.Close()
}


func (s *registry) DeleteConfig(uuid string, tbl string) error {
  _, err := s.db.Exec(`DELETE FROM ` + tbl + ` WHERE uuid=?`, uuid)
  if err != nil {
    return fmt.Errorf("DeleteConfig: %w", err)
  }
  return nil
}

func (s *registry) UpsertConfig(n *pb.NetworkConfig, sig []byte, tbl string) error {
  _, err := s.db.Exec(`
    INSERT OR REPLACE INTO ` + tbl + ` (uuid, name, description, tags, links, location, rendezvous, creator, created, signature) VALUES (?,?,?,?,?,?,?,?,?,?)`,
    n.Uuid, n.Name, n.Description,
    strings.Join(n.Tags, "\n"),
    strings.Join(n.Links, "\n"),
    n.Location, n.Rendezvous,
    n.Creator, n.Created,
    sig,
  )
  if err != nil {
    return fmt.Errorf("Upsert to registry: %w", err)
  }
  return nil
}

func (s *registry) UpsertStats(uuid string, stats *pb.NetworkStats) error {
  _, err := s.db.Exec(`
    INSERT OR REPLACE INTO stats (uuid, population, completions_last7days, records, idle_records, avg_completion_time) VALUES (?,?,?,?,?,?)`,
    uuid,
    stats.Population,
    stats.CompletionsLast7Days,
    stats.Records,
    stats.IdleRecords,
    stats.AvgCompletionTime,
  )
  if err != nil {
    return fmt.Errorf("Set stats: %w", err)
  }
  return nil
}

func (s *registry) GetLobby(ctx context.Context, cur chan<- *pb.NetworkConfig) error {
  defer close(cur)
  rows, err := s.db.Query(`SELECT * FROM "lobby"`)
  if err != nil {
    return fmt.Errorf("GetLobby SELECT: %w", err)
  }
  defer rows.Close()

  for rows.Next() {
    select {
    case <-ctx.Done():
      return fmt.Errorf("Context canceled")
    default:
    }
    c := &pb.NetworkConfig{}
    tagstr := ""
    linkstr := ""
    sig := ""
    if err := rows.Scan(
      &c.Uuid,
      &c.Name,
      &c.Description,
      &tagstr,
      &linkstr,
      &c.Location,
      &c.Rendezvous,
      &c.Creator,
      &c.Created,
      &sig); err != nil {
      return fmt.Errorf("GetNetworks scan: %w", err)
    }
    c.Tags = strings.Split(tagstr, "\n")
    c.Links = strings.Split(linkstr, "\n")
    cur<- c
  }
  return nil
}


func (s *registry) GetNetworks(ctx context.Context, cur chan<- *pb.Network, closeChan bool) error {
  if closeChan {
    defer close(cur)
  }
  rows, err := s.db.Query(`SELECT R.*, S.* FROM "registry" R LEFT JOIN "stats" S ON S.uuid=R.uuid`)
  if err != nil {
    return fmt.Errorf("GetNetwork SELECT: %w", err)
  }
  defer rows.Close()

  for rows.Next() {
    select {
    case <-ctx.Done():
      return fmt.Errorf("Context canceled")
    default:
    }
    n := &pb.Network{
      Config: &pb.NetworkConfig{},
      Stats: &pb.NetworkStats{},
    }
    tagstr := ""
    linkstr := ""
    var statuuid sql.NullString
    var statpop, statcpl7, statrec, statidle, statavgcpl sql.NullInt64

    if err := rows.Scan(
      &n.Config.Uuid,
      &n.Config.Name,
      &n.Config.Description,
      &tagstr,
      &linkstr,
      &n.Config.Location,
      &n.Config.Rendezvous,
      &n.Config.Creator,
      &n.Config.Created,
      &n.Signature,
      &statuuid,
      &statpop,
      &statcpl7,
      &statrec,
      &statidle,
      &statavgcpl); err != nil {
      return fmt.Errorf("GetNetworks scan: %w", err)
    }
    n.Config.Tags = strings.Split(tagstr, "\n")
    n.Config.Links = strings.Split(linkstr, "\n")
    n.Stats.Population = statpop.Int64
    n.Stats.CompletionsLast7Days = statcpl7.Int64
    n.Stats.Records = statrec.Int64
    n.Stats.IdleRecords = statidle.Int64
    n.Stats.AvgCompletionTime = statavgcpl.Int64
    cur<- n
  }
  return nil
}

func (s *registry) SignConfig(uuid string, sig []byte) error {
  _, err := s.db.Exec(`UPDATE stats SET signature=? WHERE uuid=?`, sig, uuid)
  if err != nil {
    return fmt.Errorf("SignConfig: %w", err)
  }
  return nil
}
