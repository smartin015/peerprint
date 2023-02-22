package www

import (
  "io/fs"
  "os"
  "context"
  "net"
  "net/http"
	"github.com/gorilla/sessions"
  "sync"
  "encoding/json"
  "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/driver"
  //"github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  "github.com/go-webauthn/webauthn/webauthn"
  "embed"
)

//go:embed static
var static embed.FS

type webserver struct {
  l *log.Sublog
  d *driver.Driver
  w *webauthn.WebAuthn
  authSession *webauthn.SessionData
  f fs.FS
  fsh http.Handler
  cs *sessions.CookieStore 
}

func New(l *log.Sublog, d *driver.Driver, liveDir string, cookieStoreKey []byte) *webserver {
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

	w, err := webauthn.New(&webauthn.Config{
			RPID: "localhost", // Must be registerable domain suffix of or equal to current domain
			RPDisplayName: "PeerPrint",
      // RPOrigins configures the list of Relying Party Server Origins that are permitted. These should be fully
      // qualified origins, i.e. with protocol, domain, and port
      RPOrigins: []string{"https://localhost:8334"},
	})
	if err != nil {
		panic(err)
	}

  return &webserver {
    l: l,
    d: d,
    f: f,
    fsh: http.FileServer(http.FS(f)),
		cs: sessions.NewCookieStore(cookieStoreKey),
		w: w,
  }
}

func (s *webserver) Serve(ctx context.Context, addr, certPath, keyPath string) {
  // Base handlers
  http.HandleFunc("/", s.WithAuth(s.handleRoot))

  // Login handlers
  http.HandleFunc("/login", s.handleLogin)
  http.HandleFunc("/login/begin", s.BeginLogin)
  http.HandleFunc("/login/finish", s.FinishLogin)
  http.HandleFunc("/logout", s.Logout)

  // Registration handlers
  http.HandleFunc("/register/begin", s.WithAuth(s.BeginRegistration))
  http.HandleFunc("/register/finish", s.WithAuth(s.FinishRegistration))
  http.HandleFunc("/register/credentials", s.WithAuth(s.handleGetCredentials))
  http.HandleFunc("/register/remove", s.WithAuth(s.handleRemoveCredentials))

  // Server stats
  http.HandleFunc("/timeline", s.WithAuth(s.handleGetTimeline))
  http.HandleFunc("/peerLogs", s.WithAuth(s.handleGetPeerLogs))
  http.HandleFunc("/events", s.WithAuth(s.handleGetEvents))
  http.HandleFunc("/serverSummary", s.WithAuth(s.handleServerSummary))
  http.HandleFunc("/storageSummary", s.WithAuth(s.handleStorageSummary))
  http.HandleFunc("/printers/location", s.WithAuth(s.handleGetPrinterLocations))
  http.HandleFunc("/printers/set_status", s.WithAuth(s.handleSetPrinterStatus))

  // Connection management
  http.HandleFunc("/connection", s.WithAuth(s.handleGetConn))
  http.HandleFunc("/connection/new", s.WithAuth(s.handleNewConn))
  http.HandleFunc("/connection/delete", s.WithAuth(s.handleDeleteConn))
  http.HandleFunc("/registry", s.WithAuth(s.handleGetRegistry))
  http.HandleFunc("/registry/new", s.WithAuth(s.handleNewRegistry))
  http.HandleFunc("/registry/delete", s.WithAuth(s.handleDeleteRegistry))
  http.HandleFunc("/lobby", s.WithAuth(s.handleGetLobby))
  http.HandleFunc("/lobby/sync", s.WithAuth(s.handleSyncLobby))

  // Server settings
  http.HandleFunc("/password/new", s.WithAuth(s.handleNewPassword))

	// TLS stuff
  l, err := net.Listen("tcp", addr)
  if err != nil {
      panic(err)
  }
  defer l.Close()

  s.l.Info("Starting status HTTP server on %s\n", l.Addr().(*net.TCPAddr).String())
	// nil args populated from http.TLSConfig

  srv := &http.Server{
  }
  
  if err := srv.ServeTLS(l, certPath, keyPath); err != nil {
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

func ErrorResponse(w http.ResponseWriter, err error) {
  w.WriteHeader(500)
  w.Write([]byte(err.Error()))
  return
}

func JSONResponse(w http.ResponseWriter, v interface{}) {
  data, err := json.Marshal(v)
  if err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
  w.Write(data)
}

func readInstance[M any](s *webserver, w http.ResponseWriter, r *http.Request, fn func(*driver.Instance) (M, error)) {
  n := s.getInstance(r, w)
  if n == nil {
    return
  }
  v, err := fn(n)
  if err != nil {
    ErrorResponse(w, err)
  } else {
    JSONResponse(w, v)
  }
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
