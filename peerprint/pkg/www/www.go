package www

import (
  "io/fs"
  "os"
  "context"
  "net"
  "net/http"
  "sync"
  "encoding/json"
  "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/driver"
  "embed"
)

//go:embed static
var static embed.FS

type webserver struct {
  l *log.Sublog
  d *driver.Driver
  f fs.FS
  fsh http.Handler
}

func New(l *log.Sublog, d *driver.Driver, liveDir string) *webserver {
  var f fs.FS
  var err error
  if liveDir == "" {
    f, err = fs.Sub(static, "static")
    if err != nil {
      panic(err)
    }
  } else {
    f = os.DirFS(liveDir)
    l.Info("Serving www assets from %s", liveDir)
  }
  return &webserver {
    l: l,
    d: d,
    f: f,
    fsh: http.FileServer(http.FS(f)),
  }
}

func (s *webserver) Serve(addr string, ctx context.Context) {
  // Base handler
  http.HandleFunc("/", basicAuth(s.d, s.handleRoot))

  // Server stats
  http.HandleFunc("/timeline", basicAuth(s.d, s.handleGetTimeline))
  http.HandleFunc("/peerLogs", basicAuth(s.d, s.handleGetPeerLogs))
  http.HandleFunc("/events", basicAuth(s.d, s.handleGetEvents))
  http.HandleFunc("/serverSummary", basicAuth(s.d, s.handleServerSummary))
  http.HandleFunc("/storageSummary", basicAuth(s.d, s.handleStorageSummary))
  http.HandleFunc("/printers/location", basicAuth(s.d, s.handleGetPrinterLocations))
  http.HandleFunc("/printers/set_status", basicAuth(s.d, s.handleSetPrinterStatus))

  // Connection management
  http.HandleFunc("/connection", basicAuth(s.d, s.handleGetConn))
  http.HandleFunc("/connection/new", basicAuth(s.d, s.handleNewConn))
  http.HandleFunc("/connection/delete", basicAuth(s.d, s.handleDeleteConn))
  http.HandleFunc("/registry", basicAuth(s.d, s.handleGetRegistry))
  http.HandleFunc("/registry/new", basicAuth(s.d, s.handleNewRegistry))
  http.HandleFunc("/registry/delete", basicAuth(s.d, s.handleDeleteRegistry))
  http.HandleFunc("/lobby", basicAuth(s.d, s.handleGetLobby))
  http.HandleFunc("/lobby/sync", basicAuth(s.d, s.handleSyncLobby))

  // Server settings
  http.HandleFunc("/password/new", basicAuth(s.d, s.handleNewPassword))

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

func (s *webserver) getInstance(r *http.Request, w http.ResponseWriter) *driver.Instance {
  n := s.d.GetInstance(r.FormValue("instance"))
  if n == nil {
    w.WriteHeader(400)
    w.Write([]byte("instance not found"))
    return nil
  }
  return n
}

func get(r *http.Request, k, d string) string {
  if v, ok := r.PostForm[k]; !ok {
    return d
  } else if len(v) == 0 {
    return d
  } else {
    return v[0]
  }
}

func readInstance[M any](s *webserver, w http.ResponseWriter, r *http.Request, fn func(*driver.Instance) (M, error)) {
  n := s.getInstance(r, w)
  if n == nil {
    return
  }
  v, err := fn(n)
  data, err := json.Marshal(v)
  if err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  w.Write(data)
}

func streamingReadInstance[M any](s *webserver, w http.ResponseWriter, r *http.Request, fn func(context.Context, *driver.Instance, chan M) error) {
  n := s.getInstance(r, w)
  if n == nil {
    return
  }

  cur := make(chan M, 5)
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
  if err := fn(ctx, n, cur); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  wg.Wait()
}
