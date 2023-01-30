// Functions for periodically cleaning up the DB
package storage

import (
  "fmt"
  "time"
  _ "github.com/mattn/go-sqlite3"
)

const (
  CompletedRecordTTL = 24*time.Hour
)

func(s *sqlite3) deleteCompletedRecords() error {
  // If an approver's signed record is matched by the approver's 
  // signed completion (with timestamp) we consider the record complete.
  // It's persisted for awhile to allow the completion to propagate to peers,
  // but eventually it's garbage-collected
  _, err := s.db.Exec(`
    DELETE FROM "records" WHERE (uuid, signer) IN (
      SELECT R.uuid, R.signer
        FROM "records" R
        LEFT JOIN "completions" C
        ON R.uuid=C.uuid 
        AND R.signer=C.signer
      WHERE 
        C.timestamp > 0 AND C.timestamp < ?
        AND R.signer = R.approver
    )
  `, time.Now().Add(-CompletedRecordTTL).Unix())
  return err
}

func (s *sqlite3) Cleanup() []error {
  s.lastCleanupStart = time.Now()
  defer func() {
    s.lastCleanupEnd = time.Now()
  }()
  ee := []error{}

  if err := s.deleteCompletedRecords(); err != nil {
    ee = append(ee, fmt.Errorf("Clearing completed records: %w", err))
  }
  return ee
}

