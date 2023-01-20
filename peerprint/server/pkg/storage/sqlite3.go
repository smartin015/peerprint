package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
	"github.com/libp2p/go-libp2p/core/crypto"
  "database/sql"
  "strings"
  "fmt"
  "time"
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

func (s *sqlite3) migrateSchema() error {
  tx, err := s.db.Begin()
  if err != nil {
    return err
  }
  defer tx.Rollback()

  if _, err := tx.Exec(`
  CREATE TABLE grants (
    target TEXT NOT NULL,
    expiry INT NOT NULL,
    type INT NOT NULL,
    scope TEXT NOT NULL,

    signer BLOB NOT NULL,
    signature BLOB NOT NULL,

    PRIMARY KEY (target, type, scope, signer)
  );`); err != nil {
    return fmt.Errorf("create grants table: %w", err)
  }

  if _, err := tx.Exec(`
  CREATE TABLE recordtags (
    name TEXT NOT NULL,
    record TEXT NOT NULL,
    version INT NOT NULL,
    PRIMARY KEY (name, record, version)
  );`); err != nil {
    return fmt.Errorf("create recordtags table: %w", err)
  }

  if _, err := tx.Exec(`
  CREATE TABLE records (
    uuid TEXT NOT NULL,
    location TEXT NOT NULL,
    version INT NOT NULL,
    created INT NOT NULL,
    tombstone INT,
  
    num INT NOT NULL,
    den INT NOT NULL,
    gen REAL NOT NULL,

    signer BLOB NOT NULL,
    signature BLOB NOT NULL
  );`); err != nil {
    return fmt.Errorf("create records table: %w", err)
  }

  if _, err := tx.Exec(`
  CREATE TABLE pubkeys (
    peer TEXT NOT NULL PRIMARY KEY,
    key BLOB NOT NULL
  );`); err != nil {
    fmt.Errorf("create pubkeys table: %w", err)
  }
  return tx.Commit()
}

func (s *sqlite3) SetSignedRecord(r *pb.SignedRecord) error {
  tx, err := s.db.Begin()
  if err != nil {
    return err
  }
  defer tx.Rollback()

  // Wipe out and repopulate tags
  if _, err := tx.Exec(`DELETE FROM recordtags WHERE record=$1 AND version=$2`, r.Record.Uuid, r.Record.Version); err != nil {
    return fmt.Errorf("delete tags: %w", err)
  }

  stmt, err := tx.Prepare(`INSERT INTO recordtags (name, record, version) VALUES (?, ?, ?)`)
  if err != nil {
    return fmt.Errorf("prepare tags: %w", err)
  }
  defer stmt.Close()
  for _, tag := range r.Record.Tags {
    if _, err := stmt.Exec(tag, r.Record.Uuid, r.Record.Version); err != nil {
      return fmt.Errorf("insert tag %s: %w", tag, err)
    }
  }

  if _, err = tx.Exec(`
    INSERT INTO records (uuid, location, version, created, tombstone, num, den, gen, signer, signature)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);
  `, r.Record.Uuid, r.Record.Location, r.Record.Version, r.Record.Created, r.Record.Tombstone, r.Record.Rank.Num, r.Record.Rank.Den, r.Record.Rank.Gen, r.Signature.Signer, r.Signature.Data); err != nil {
    return fmt.Errorf("insert into records: %w", err)
  }

  // TODO garbage collect other records of similar type
  return tx.Commit()
}


type scannable interface {
  Scan(...any) error
}

func scanSignedRecord(r scannable, result *pb.SignedRecord) error {
  return r.Scan(
    &result.Record.Uuid,
    &result.Record.Location,
    &result.Record.Version,
    &result.Record.Created,
    &result.Record.Tombstone,
    &result.Record.Rank.Num,
    &result.Record.Rank.Den,
    &result.Record.Rank.Gen,
    &result.Signature.Signer,
    &result.Signature.Data,
  );
}

func scanSignedGrant(r scannable, result *pb.SignedGrant) error {
  return r.Scan(
    &result.Grant.Target,
    &result.Grant.Expiry,
    &result.Grant.Type,
    &result.Grant.Scope,
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
  if err := scanSignedRecord(s.db.QueryRow(`SELECT * FROM "records" WHERE uuid=$1 AND tombstone=0 ORDER BY version DESC LIMIT 1;`, uuid), result); err != nil {
    if err == sql.ErrNoRows {
      return err // Allow for comparison upstream
    }
    return fmt.Errorf("select record: %w", err)
  }

  // Import tags now that we have a version
  rows, err := s.db.Query(`SELECT name FROM recordtags WHERE record=$1 AND version=$2`, uuid, result.Record.Version)
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

func (s *sqlite3) GetSignedRecords(opts ...any) ([]*pb.SignedRecord, error) {
  result := []*pb.SignedRecord{}
  where := []string{}
  args := []any{}
  for _, opt := range opts {
    switch v := opt.(type) {
    case WithUUID:
      where = append(where,  "uuid=?")
      args = append(args, string(v))
    default:
        return nil, fmt.Errorf("GetSignedRecords received invalid option: %v", opt)
    }
  }
  q := `SELECT * FROM "records" WHERE tombstone=0 `
  if len(where) > 0 {
    q += strings.Join(where, " AND ")
  }
  q += "GROUP BY uuid HAVING version=MAX(version);"
  rows, err := s.db.Query(q, args...)
  if err != nil {
    return nil, fmt.Errorf("GetSignedRecords SELECT: %w", err)
  }
  
  for rows.Next() {
    sr := &pb.SignedRecord{Signature: &pb.Signature{}, Record: &pb.Record{Rank: &pb.Rank{}}}
    if err := scanSignedRecord(rows, sr); err != nil {
      rows.Close()
      return nil, fmt.Errorf("GetSignedRecords scan: %w", err)
    }
    result = append(result, sr)
  }
  rows.Close()
  return result, nil
}

func (s *sqlite3) SetSignedGrant(g *pb.SignedGrant) error {
  // Going out on a limb here and saying if this grant is being issued here,
  // it's got a later expiry than any other grants of the same type etc.
  // This means we can use INSERT OR REPLACE here
  _, err := s.db.Exec(`
    INSERT OR REPLACE INTO "grants" (target, expiry, type, scope, signer, signature)
    VALUES ($1, $2, $3, $4, $5, $6);
  `, g.Grant.Target, g.Grant.Expiry, g.Grant.Type, g.Grant.Scope, g.Signature.Signer, g.Signature.Data)
  return err
}

func (s *sqlite3) GetRandomGrantWithScope() (*pb.SignedGrant, error) {
  result := &pb.SignedGrant{Signature: &pb.Signature{}, Grant: &pb.Grant{}}
  if err := scanSignedGrant(s.db.QueryRow(`SELECT * FROM "grants" WHERE scope != "" ORDER BY RANDOM() LIMIT 1;`), result); err != nil {
    if err == sql.ErrNoRows {
      return nil, err // Allow for comparison upstream
    }
    return nil, fmt.Errorf("get random record: %w", err)
  }
  return result, nil
}

func (s *sqlite3) GetSignedGrants(opts ...any) ([]*pb.SignedGrant, error) {
  result := []*pb.SignedGrant{}
  expired := false
  where := []string{}
  args := []any{}
  for _, opt := range opts {
    switch v := opt.(type) {
    case WithType:
      where = append(where,  "type=?")
      args = append(args, int(v))
    case WithTarget:
      where = append(where, "target=?")
      args = append(args, string(v))
    case WithScope:
      where = append(where, "scope=?")
      args = append(args, string(v))
    case IncludeExpired:
      expired =  bool(v)
    default:
        return nil, fmt.Errorf("GetSignedGrants received invalid option: %v", opt)
    }
  }
  if !expired {
    where = append(where, "expiry > ?")
    args = append(args, time.Now().Unix())
  }
  q := `SELECT * FROM "grants"`
  if len(where) > 0 {
    q += " WHERE " + strings.Join(where, " AND ")
  }
  q += ";"
  rows, err := s.db.Query(q, args...)
  if err != nil {
    return nil, fmt.Errorf("%s: %w", q, err)
  }
  for rows.Next() {
    sr := &pb.SignedGrant{Grant: &pb.Grant{}, Signature: &pb.Signature{}}
    if err := scanSignedGrant(rows, sr); err != nil {
      rows.Close()
      return nil, fmt.Errorf("GetSignedGrants scan: %w", err)
    }
    result = append(result, sr)
  }
  rows.Close()
  return result, nil
}

func (s *sqlite3) SetPubKey(peer string, pubkey crypto.PubKey) error {
  b, err := crypto.MarshalPublicKey(pubkey)
  if err != nil {
    return fmt.Errorf("Unwrap key: %w", err)
  }
  if _, err = s.db.Exec(`
    INSERT OR REPLACE INTO "pubkeys" (peer, key)
    VALUES ($1, $2);
  `, peer, b); err != nil {
    return fmt.Errorf("SetPubKey: %w", err)
  }
  return nil
}

func (s *sqlite3) GetPubKey(peer string) (crypto.PubKey, error) {
  var b []byte
  if err := s.db.QueryRow(`SELECT key FROM "pubkeys" WHERE peer=$1 LIMIT 1;`, peer).Scan(&b); err != nil {
    return nil, err
  }
  return crypto.UnmarshalPublicKey(b)
}

func (s *sqlite3) IsAdmin(peer string) (bool, error) {
  admin := false
  err := s.db.QueryRow(`SELECT COUNT(*)>0 FROM "grants" WHERE target=$1 AND expiry > $2 AND type=$3;`, peer, time.Now().Unix(), pb.GrantType_ADMIN).Scan(&admin)
  return admin, err
}

func (s *sqlite3) CountAdmins() (int64, error) {
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(*) FROM "grants" WHERE expiry > $1 AND type=$2 GROUP BY target;`, time.Now().Unix(), pb.GrantType_ADMIN).Scan(&num)
  return num, err
}

func (s *sqlite3) CountRecords() (int64, error) {
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(*) FROM "records" WHERE tombstone=0;`).Scan(&num)
  return num, err
}

func (s *sqlite3) ValidGrants() (bool, error) {
  anyValid := false
  err := s.db.QueryRow(`SELECT COUNT(*)>0 FROM "grants" WHERE expiry > $1;`, time.Now().Unix()).Scan(&anyValid)
  return anyValid, err
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
