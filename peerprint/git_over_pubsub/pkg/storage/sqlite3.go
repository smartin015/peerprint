package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
	"github.com/libp2p/go-libp2p/core/crypto"
  "database/sql"
  "fmt"
  "time"
  _ "github.com/mattn/go-sqlite3"
)

type sqlite3 struct {
  db *sql.DB
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
    signature BLOB NOT NULL
  );`); err != nil {
    return fmt.Errorf("create grants table: %w", err)
  }

  if _, err := tx.Exec(`
  CREATE TABLE tags (
    tag TEXT NOT NULL,
    record TEXT NOT NULL
    PRIMARY KEY (tag, record)
  );`); err != nil {
    fmt.Errorf("create tags table: %w", err)
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
  );
  CREATE TABLE pubkeys (
    peer TEXT NOT NULL PRIMARY KEY,
    key BLOB NOT NULL
  );`); err != nil {
    fmt.Errorf("create records table: %w", err)
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
  if _, err := s.db.Exec(`DELETE FROM tags WHERE record=$1`, r.Record.Uuid); err != nil {
    return err
  }
   stmt, err := s.db.Prepare("INSERT INTO tags (tag, record) VALUES ($1, $2) ON CONFLICT IGNORE")
   defer stmt.Close()
  if err != nil {
    return err
  }
  for _, tag := range r.Record.Tags {
    if _, err := stmt.Exec(tag, r.Record.Uuid); err != nil {
      return err
    }
  }

  if _, err = tx.Exec(`
    INSERT INTO records (uuid, location, version, created, tombstone, num, den, gen, signer, signature)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);
  `, r.Record.Uuid, r.Record.Location, r.Record.Version, r.Record.Created, r.Record.Tombstone, r.Record.Rank.Num, r.Record.Rank.Den, r.Record.Rank.Gen, r.Signature.Signer, r.Signature.Data); err != nil {
    return err
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

func (s *sqlite3) GetSignedRecord(uuid string, result *pb.SignedRecord) error {
  if err := scanSignedRecord(s.db.QueryRow(`SELECT * FROM records WHERE uuid=$1 AND tombstone IS NOT NULL ORDER BY version LIMIT 1;`), result); err != nil {
    return err
  }
  return nil
}

func (s *sqlite3) GetSignedRecords() ([]*pb.SignedRecord, error) {
  result := []*pb.SignedRecord{}
  rows, err := s.db.Query(`SELECT * FROM records WHERE tombstone IS NOT NULL GROUP BY uuid HAVING version=MAX(version);`)
  if err != nil {
    return nil, fmt.Errorf("GetSignedRecords SELECT: %w", err)
  }
  for rows.Next() {
    sr := &pb.SignedRecord{Signature: &pb.Signature{}, Record: &pb.Record{Rank: &pb.Rank{}}}
    if err := scanSignedRecord(rows, sr); err != nil {
      return nil, fmt.Errorf("GetSignedRecords scan: %w", err)
    }
    result = append(result, sr)
  }
  return result, nil
}

func (s *sqlite3) SetSignedGrant(g *pb.SignedGrant) error {
  _, err := s.db.Exec(`
    INSERT INTO grants (target, expiry, type, scope, signer, signature)
    VALUES ($1, $2, $3, $4, $5, $6);
  `, g.Grant.Target, g.Grant.Expiry, g.Grant.Type, g.Grant.Scope, g.Signature.Signer, g.Signature.Data)
  return err
}

func (s *sqlite3) GetSignedGrants() ([]*pb.SignedGrant, error) {
  result := []*pb.SignedGrant{}
  rows, err := s.db.Query(`SELECT * FROM grants WHERE expiry > $1;`, time.Now().Unix())
  if err != nil {
    return nil, fmt.Errorf("GetSignedGrants SELECT: %w", err)
  }
  for rows.Next() {
    sr := &pb.SignedGrant{Grant: &pb.Grant{}, Signature: &pb.Signature{}}
    if err := scanSignedGrant(rows, sr); err != nil {
      return nil, fmt.Errorf("GetSignedGrants scan: %w", err)
    }
    result = append(result, sr)
  }
  return result, nil
}

func (s *sqlite3) SetPubKey(peer string, pubkey crypto.PubKey) error {
  b, err := crypto.MarshalPublicKey(pubkey)
  if err != nil {
    return fmt.Errorf("Unwrap key: %w", err)
  }
  if _, err = s.db.Exec(`
    INSERT OR REPLACE INTO pubkeys (peer, key)
    VALUES ($1, $2);
  `, peer, b); err != nil {
    return fmt.Errorf("SetPubKey: %w", err)
  }
  return nil
}

func (s *sqlite3) GetPubKey(peer string) (crypto.PubKey, error) {
  var b []byte
  if err := s.db.QueryRow(`SELECT key FROM pubkeys WHERE peer=$1 LIMIT 1;`).Scan(&b); err != nil {
    return nil, err
  }
  return crypto.UnmarshalPublicKey(b)
}

func (s *sqlite3) IsAdmin(peer string) (bool, error) {
  admin := false
  err := s.db.QueryRow(`SELECT COUNT(*)=1 FROM grants WHERE target=$1 AND expiry > $2 AND type=$3;`, peer, time.Now().Unix(), pb.GrantType_ADMIN).Scan(&admin)
  return admin, err
}

func (s *sqlite3) CountAdmins() (int, error) {
  num := 0 
  err := s.db.QueryRow(`SELECT COUNT(*) FROM grants WHERE expiry > $1 AND type=$2 GROUP BY target;`, time.Now().Unix(), pb.GrantType_ADMIN).Scan(&num)
  return num, err
}

func (s *sqlite3) ValidGrants() (bool, error) {
  anyValid := false
  err := s.db.QueryRow(`SELECT COUNT(*)>0 FROM grants WHERE expiry > $1;`, time.Now().Unix()).Scan(&anyValid)
  return anyValid, err
}

func NewSqlite3(path string) (*sqlite3, error) {
  db, err := sql.Open("sqlite3", path)
  if err != nil {
    return nil, fmt.Errorf("failed to open db at %s: %w", path, err)
  }
  s := &sqlite3{
    db: db,
  }
  if err := s.migrateSchema(); err != nil {
    return nil, fmt.Errorf("failed to migrate schema: %w", err)
  }
  return s, nil
}
