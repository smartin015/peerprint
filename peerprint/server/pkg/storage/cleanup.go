// Functions for periodically cleaning up the DB
package storage

import (
  "fmt"
  "time"
  _ "github.com/mattn/go-sqlite3"
)

const (
  OldTrustThresh = 5*time.Minute
  OldWorkabilityThresh = 5*time.Minute
  CompletedRecordTTL = 24*time.Hour
  PeerTrustThresh = 0.5
)

func (s *sqlite3) deleteRecordsUntilSignerCount(trustThresh float64, untilPeers int64) error {
  n := int64(0)
  if err := s.db.QueryRow(`SELECT COUNT(DISTINCT signer) FROM "records"`).Scan(&n); err != nil {
    return fmt.Errorf("Count record signers: %w", err)
  }
  ncmp := int64(0)
  if err := s.db.QueryRow(`SELECT COUNT(DISTINCT signer) FROM "completions"`).Scan(&ncmp); err != nil {
    return fmt.Errorf("Count completion signers: %w", err)
  } else if ncmp > n {
    n = ncmp
  }
  want :=  n - untilPeers
  if want <= 0 {
    return nil
  }
  // Remove trusted and untrusted peers alike:
  //    - trusted peers where their most recent message is too old
  //   - untrusted peers where their first message is too old
  // Note: record deletions cascade to completions, but records may have
  // peer completions that we want deleted as well.
  rows, err := s.db.Query(`
      SELECT 
        peer,
        IF(trust > ?,
          last_seen,
          first_seen,
        ) AS recency,
        FROM "trust" R
      GROUP BY T1.signer
      ORDER BY age ASC
      LIMIT ?
  `, trustThresh, want)
  if err != nil {
    return fmt.Errorf("gather candidate peer IDs: %w", err)
  }
  defer rows.Close()
  for rows.Next() {
    peer := ""
    recency := int64(0)
    if err := rows.Scan(&peer, &recency); err != nil {
      return fmt.Errorf("scan: %w", err)
    }
    if _, err := s.db.Exec(`DELETE FROM "records" WHERE signer=? OR approver=?`); err != nil {
      return fmt.Errorf("delete %s from records: %w", peer, err)
    }
    if _, err := s.db.Exec(`DELETE FROM "completions" WHERE signer=? OR completer=?`); err != nil {
      return fmt.Errorf("delete %s from completions: %w", peer, err)
    }
  }
  return nil
}

func(s *sqlite3) deleteCompletedRecords() error {
  _, err := s.db.Exec(`
    DELETE FROM "records" WHERE (uuid, signer) IN (
      SELECT R.uuid, R.signer
        FROM "records" R
        LEFT JOIN "completions" C
        ON R.uuid=C.uuid 
        AND R.signer=C.signer
      WHERE 
        C.timestamp > 0
        AND R.signer = R.approver
    )
  `)
  return err
}

func (s *sqlite3) deleteOldWorkability() error {
  _, err := s.db.Exec(`
    DELETE FROM "workability" WHERE uuid IN (
      SELECT C.uuid FROM "completions" C
        LEFT JOIN "records" R
        ON (C.uuid=R.uuid AND C.signer=R.signer)
      WHERE R.approver != NULL AND R.approver=R.signer
    )
  `)
  return err
}

func (s *sqlite3) Cleanup(untilPeers int64) []error {
  s.lastCleanupStart = time.Now()
  defer func() {
    s.lastCleanupEnd = time.Now()
  }()
  ee := []error{}

  if err := s.deleteRecordsUntilSignerCount(PeerTrustThresh, untilPeers); err != nil {
    ee = append(ee, fmt.Errorf("Culling peers: %w", err))
  }
  if err := s.deleteCompletedRecords(); err != nil {
    ee = append(ee, fmt.Errorf("Clearing completed records: %w", err))
  }
  if err := s.deleteOldWorkability(); err != nil {
    ee = append(ee, fmt.Errorf("Cleaning up workability: %w", err))
  }
  return ee
}

