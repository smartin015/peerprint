package storage

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "context"
  "fmt"
  "time"
  //"log"
  _ "github.com/mattn/go-sqlite3"
  _ "embed"
)

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

func (s *sqlite3) SetPeerStatus(peer string, status *pb.PeerStatus) error {
  for _, p := range status.Printers {
    _, err := s.db.Exec(`
      INSERT OR REPLACE INTO printers (server, server_name, name, active_record, active_unit, status, profile, latitude, longitude, timestamp) VALUES (
      ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, peer, status.Name, p.Name, p.ActiveRecord, p.ActiveUnit, p.Status, p.Profile, p.Location.Latitude, p.Location.Longitude, p.Timestamp)
    if err != nil {
      return err
    }
  }
  return nil
}

func (s *sqlite3) GetPrinterLocations(ctx context.Context, after int64, cur chan<- *pb.Location) error {
  defer close(cur)
  q := `SELECT latitude, longitude FROM "printers" WHERE timestamp>?;`
  rows, err := s.db.Query(q, &after)
  if err != nil {
    return fmt.Errorf("GetPrinterLocations SELECT: %w", err)
  }
  defer rows.Close()
  for rows.Next() {
    select {
    case <-ctx.Done():
      return fmt.Errorf("Context canceled")
    default:
    }
    l := &pb.Location{}
    if err := rows.Scan(&l.Latitude, &l.Longitude); err != nil {
      return fmt.Errorf("GetPrinterLocations scan: %w", err)
    }
    cur<- l
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
