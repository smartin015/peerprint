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

func (s *sqlite3) countCompletionSigners(exclude string) (int64, error) {
  num := int64(0)
  err := s.db.QueryRow(`SELECT COUNT(DISTINCT(signer)) FROM "completions" WHERE signer != ?;`, exclude).Scan(&num)
  if err == sql.ErrNoRows {
    return 0, nil
  }
  return num, err
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
    case WithUUID:
      where = append(where,  "uuid=?")
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

