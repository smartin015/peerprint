// Functions for periodically cleaning up the DB
package storage

import (
  "fmt"
  "time"
  _ "github.com/mattn/go-sqlite3"
)

func(s *sqlite3) deleteCompletedRecords(amt int64) (int64, error) {
  n, err := s.countCompletedRecords()
  if err != nil {
    return 0, err
  }

  n -= amt
  if n <= 0 {
    return 0, nil
  }

  // If an approver's signed record is matched by the approver's 
  // signed completion (with timestamp) we consider the record complete.
  // It's persisted for awhile to allow the completion to propagate to peers,
  // but eventually it's garbage-collected
  res, err := s.db.Exec(`
    DELETE FROM "records" WHERE (uuid, signer) IN (
      SELECT R.uuid, R.signer
        FROM "records" R
        LEFT JOIN "completions" C
        ON R.uuid=C.uuid 
        AND R.signer=C.signer
      WHERE 
        C.timestamp > 0
        AND R.signer = R.approver
      ORDER BY C.timestamp ASC
      LIMIT ?
    )
  `, n)
  if err != nil {
    return 0, err
  }

  if num, err := res.RowsAffected(); err != nil {
    return 0, err
  } else {
    return num, nil
  }
}

func (s *sqlite3) deleteDanglingCompletions() (int64, error) {
  res, err := s.db.Exec(`
    DELETE FROM "completions" WHERE (uuid, signer) IN (
      SELECT C.uuid, C.signer
        FROM "completions" C
        LEFT JOIN "records" R
          ON C.uuid=R.uuid 
          AND C.signer=R.signer
        WHERE R.uuid IS NULL
    )
  `)
  if err != nil {
    return 0, err
  } else if num, err := res.RowsAffected(); err != nil {
    return 0 ,err
  } else {
    return num, nil
  }
}

func (s *sqlite3) Cleanup(until_records int64) (int64, error) {
  s.lastCleanupStart = time.Now()
  defer func() {
    s.lastCleanupEnd = time.Now()
  }()
  n1, err := s.deleteCompletedRecords(until_records)
  if err != nil {
    return n1, fmt.Errorf("delete completed records: %w", err)
  }
  n2, err := s.deleteDanglingCompletions()
  if err != nil {
    return n1, fmt.Errorf("delete dangling completions: %w", err)
  }
  return n1+n2, nil
}
