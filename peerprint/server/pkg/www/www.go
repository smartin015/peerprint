package www

import (
  "sync"
  "io/fs"
  "context"
  "time"
  "net"
  "net/http"
  "encoding/json"
  "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "github.com/smartin015/peerprint/p2pgit/pkg/driver"
  "embed"
)

const (
  DBReadTimeout = 5*time.Second
)

//go:embed static
var static embed.FS

type webserver struct {
  l *log.Sublog
  d *driver.Driver 
}

func New(l *log.Sublog, d *driver.Driver) *webserver {
  return &webserver {
    l: l,
    d: d,
  }
}

func (s *webserver) getInstance(r *http.Request, w http.ResponseWriter) *driver.Instance {
  n := s.d.GetInstance(r.FormValue("instance"))
  if n == nil {
    w.WriteHeader(404)
    w.Write([]byte("instance not found"))
    return nil
  }
  return n
}

func (s *webserver) handleGetEvents(w http.ResponseWriter, r *http.Request) {
  n := s.getInstance(r, w)
  if n == nil {
    return
  }

  cur := make(chan storage.DBEvent, 5)
  var wg sync.WaitGroup
  wg.Add(1)
  go func() {
    defer wg.Done()
    for v := range cur {
      if data, err := json.Marshal(v); err != nil {
        w.WriteHeader(500)
        w.Write([]byte(err.Error()))
        return
      } else {
        w.Write(data)
        w.Write([]byte("\n"))
      }
    }
  }()
  ctx, _ := context.WithTimeout(context.Background(), DBReadTimeout)
  if err := n.St.GetEvents(ctx, cur, 1000); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  wg.Wait()
}

func (s *webserver) handleGetTimeline(w http.ResponseWriter, r *http.Request) {
  n := s.getInstance(r, w)
  if n == nil {
    return
  }
  cur := make(chan *storage.DataPoint, 5)
  var wg sync.WaitGroup
  wg.Add(1)
  go func() {
    defer wg.Done()
    for v := range cur {
      if data, err := json.Marshal(v); err != nil {
        w.WriteHeader(500)
        w.Write([]byte(err.Error()))
        return
      } else {
        w.Write(data)
        w.Write([]byte("\n"))
      }
    }
  }()
  ctx, _ := context.WithTimeout(context.Background(), DBReadTimeout)
  if err := n.St.GetPeerTimeline(ctx, cur); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  wg.Wait()
}

func (s *webserver) handleGetInstances(w http.ResponseWriter, r *http.Request) {
  v := s.d.InstanceNames()
  if data, err := json.Marshal(v); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
    return
  } else {
    w.Write(data)
  }
}

func (s *webserver) handleGetPeerLogs(w http.ResponseWriter, r *http.Request) {
  n := s.getInstance(r, w)
  if n == nil {
    return
  }
  cur := make(chan *storage.TimeProfile, 5)
  var wg sync.WaitGroup
  wg.Add(1)
  go func() {
    defer wg.Done()
    for v := range cur {
      if data, err := json.Marshal(v); err != nil {
        w.WriteHeader(500)
        w.Write([]byte(err.Error()))
        return
      } else {
        w.Write(data)
        w.Write([]byte("\n"))
      }
    }
  }()
  ctx, _ := context.WithTimeout(context.Background(), DBReadTimeout)
  if err := n.St.GetPeerTracking(ctx, cur); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  wg.Wait()
}

func (s *webserver) handleServerSummary(w http.ResponseWriter, r *http.Request) {
  n  := s.getInstance(r, w)
  if n == nil {
    return
  }
  data, err := json.Marshal(n.S.GetSummary())
  if err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  w.Write(data)
}

func (s *webserver) handleStorageSummary(w http.ResponseWriter, r *http.Request) {
  n := s.getInstance(r, w)
  if n == nil {
    return
  }
  summary, errs := n.St.GetSummary()
  for _, e := range errs {
    s.l.Error("handleStorageSummary: %v", e)
  }
  data, err := json.Marshal(summary)
  if err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  w.Write(data)
}

func (s *webserver) Serve(addr string, ctx context.Context, liveDir string) {
  if liveDir == "" {
    subFS, err := fs.Sub(static, "static")
    if err != nil {
      panic(err)
    }
    http.Handle("/", http.FileServer(http.FS(subFS)))
  } else {
    http.Handle("/", http.FileServer(http.Dir(liveDir)))
    s.l.Info("Serving www assets from %s", liveDir)
  }

  http.HandleFunc("/instances", s.handleGetInstances)
  http.HandleFunc("/timeline", s.handleGetTimeline)
  http.HandleFunc("/peerLogs", s.handleGetPeerLogs)
  http.HandleFunc("/events", s.handleGetEvents)
  http.HandleFunc("/serverSummary", s.handleServerSummary)
  http.HandleFunc("/storageSummary", s.handleStorageSummary)

  l, err := net.Listen("tcp", addr)
  if err != nil {
      panic(err)
  }
  defer l.Close()

  s.l.Info("Starting status HTTP server on %s\n", l.Addr().(*net.TCPAddr).String())
  if err := http.Serve(l, nil); err != nil {
    s.l.Fatal(err)
  }
}
