package storage

import (
  "strings"
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
    if p.Location == nil {
      p.Location = &pb.Location{} // Prevent nil access
    }
    _, err := s.db.Exec(`
      INSERT OR REPLACE INTO printers (server, server_name, name, active_record, active_unit, status, profile, latitude, longitude, timestamp) VALUES (
      ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, peer, status.Name, p.Name, p.ActiveRecord, p.ActiveUnit, p.Status, p.Profile, p.Location.Latitude, p.Location.Longitude, p.Timestamp)
    if err != nil {
      return err
    }
  }
  return nil
}

func scanPrinter(r scannable, serverId *string, serverName *string, result *pb.PrinterStatus) error {
  result.Location = &pb.Location{}
  return r.Scan(
    serverId,
    serverName,
    &result.Name,
    &result.ActiveRecord,
    &result.ActiveUnit,
    &result.Status,
    &result.Profile,
    &result.Location.Latitude,
    &result.Location.Longitude,
    &result.Timestamp,
  )
}

func (s *sqlite3) GetPeerStatuses(ctx context.Context, cur chan<- *pb.PeerStatus, opts ...any) error {
  defer close(cur)
  where := []string{}
  args := []any{}
  limit := -1
  for _, opt := range opts {
    switch v := opt.(type) {
    case AfterTimestamp:
      where = append(where,  "timestamp>?")
      args = append(args, int64(v))
    case WithSigner:
      where = append(where, "server=?")
      args = append(args, string(v))
    case WithLimit:
      limit = int(v)
    default:
      return fmt.Errorf("GetPeerStatuses received invalid option: %v", opt)
    }
  }
  q := `SELECT * FROM "printers" `
  if len(where) > 0 {
    q += "WHERE " + strings.Join(where, " AND ")
  }
  q += "ORDER BY server" // Allow us to group via for loop
  if limit > 0 {
    q += fmt.Sprintf(" LIMIT %d", limit)
  }
  rows, err := s.db.Query(q, args...)
  if err != nil {
    return fmt.Errorf("GetPeerStatuses SELECT: %w", err)
  }
  defer rows.Close()
  acc := &pb.PeerStatus{Printers: []*pb.PrinterStatus{}}
  curSid := ""
  for rows.Next() {
    select {
    case <-ctx.Done():
      return fmt.Errorf("Context canceled")
    default:
    }
    sid := ""
    ps := &pb.PrinterStatus{}
    if err := scanPrinter(rows, &sid, &acc.Name, ps); err != nil {
      return fmt.Errorf("GetPrinterLocations scan: %w", err)
    } else if sid == curSid {
      acc.Printers = append(acc.Printers, ps)
    } else {
      if curSid != "" {
        cur<- acc
      }
      acc = &pb.PeerStatus{Printers: []*pb.PrinterStatus{ps}}
      curSid = sid
    }
  }
  if len(acc.Printers) != 0 {
    cur<- acc
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
