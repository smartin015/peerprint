package www

import (
  "strconv"
  "strings"
  "sync"
  "io/fs"
  "os"
  "context"
  "html/template"
  "time"
  "net"
  "net/http"
  "encoding/json"
  "google.golang.org/protobuf/encoding/protojson"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/google/uuid"
  "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "github.com/smartin015/peerprint/p2pgit/pkg/driver"
  "crypto/sha256"
  "crypto/subtle"
  "embed"
)

const (
  DBReadTimeout = 5*time.Second
)

var (
  username="admin"
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

// https://www.alexedwards.net/blog/basic-authentication-in-go
func basicAuth(d *driver.Driver, next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Extract the username and password from the request 
    // Authorization header. If no Authentication header is present 
    // or the header value is invalid, then the 'ok' return value 
    // will be false.
		username, password, ok := r.BasicAuth()
		if ok {
      // Calculate SHA-256 hashes for the provided and expected
      // usernames and passwords.
      wantPassHash, salt := d.GetAdminPassAndSalt()
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256(append([]byte(password), salt...))
			expectedUsernameHash := sha256.Sum256([]byte(username))

      // Use the subtle.ConstantTimeCompare() function to check if 
      // the provided username and password hashes equal the  
      // expected username and password hashes. ConstantTimeCompare
      // will return 1 if the values are equal, or 0 otherwise. 
      // Importantly, we should to do the work to evaluate both the 
      // username and password before checking the return values to 
      // avoid leaking information.
			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], wantPassHash[:]) == 1)

      // If the username and password are correct, then call
      // the next handler in the chain. Make sure to return 
      // afterwards, so that none of the code below is run.
			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, r)
				return
			}
		}

    // If the Authentication header is not present, is invalid, or the
    // username or password is wrong, then set a WWW-Authenticate 
    // header to inform the client that we expect them to use basic
    // authentication and send a 401 Unauthorized response.
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
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


func (s *webserver) handleIndex(w http.ResponseWriter, r *http.Request) {
  tmpl, err := template.ParseFS(s.f, "*.html")
  if err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  if err := tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
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

func (s *webserver) handleSyncLobby(w http.ResponseWriter, r *http.Request) {
  d, err := strconv.Atoi(r.FormValue("seconds"))
  if err != nil {
    w.WriteHeader(400)
    w.Write([]byte("failed to parse seconds"))
    return
  }
  dt := time.Duration(d) * time.Second
  go func() {
    if err := s.d.RLocal.Run(dt); err != nil {
      s.l.Error("Local sync: %s", err.Error())
    }
  }()
  go func() {
    if err := s.d.RWorld.Run(dt); err != nil {
      s.l.Error("Global sync: %s", err.Error())
    }
  }()
  w.Write([]byte("ok"))
}

func (s *webserver) handleGetLobby(w http.ResponseWriter, r *http.Request) {
  cur := make(chan *pb.Network, 5)

  if v, err := json.Marshal(s.d.RLocal.Counters); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  } else {
    w.Write(v)
    w.Write([]byte("\n"))
  }
  if v, err := json.Marshal(s.d.RWorld.Counters); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  } else {
    w.Write(v)
    w.Write([]byte("\n"))
  }

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
  if err := s.d.RLocal.DB.GetRegistry(ctx, cur, storage.LobbyTable, false); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  if err := s.d.RWorld.DB.GetRegistry(ctx, cur, storage.LobbyTable, true); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  wg.Wait()
}

func (s *webserver) handleGetRegistry(w http.ResponseWriter, r *http.Request) {
  cur := make(chan *pb.Network, 5)
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
  if err := s.d.RLocal.DB.GetRegistry(ctx, cur, storage.RegistryTable, false); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  if err := s.d.RWorld.DB.GetRegistry(ctx, cur, storage.RegistryTable, true); err != nil {
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

func (s *webserver) handleGetConn(w http.ResponseWriter, r *http.Request) {
  v := s.d.GetConfigs(true)
  for _, c := range v {
    j := &protojson.MarshalOptions{EmitUnpopulated: true}
    if data, err := j.Marshal(c); err != nil {
      w.WriteHeader(500)
      w.Write([]byte(err.Error()))
      return
    } else {
      w.Write(data)
      w.Write([]byte("\n"))
    }
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

func get(r *http.Request, k, d string) string {
  if v, ok := r.PostForm[k]; !ok {
    return d
  } else if len(v) == 0 {
    return d
  } else {
    return v[0]
  }
}

func (s *webserver) handleDeleteConn(w http.ResponseWriter, r *http.Request) {
  if v := r.PostFormValue("network"); v == "" {
      w.WriteHeader(400)
      w.Write([]byte("Missing network"))
      return
  } else {
    req := &pb.DisconnectRequest{Network: v}
    if _, err := s.d.Command.Disconnect(r.Context(), req); err != nil {
      w.WriteHeader(500)
      w.Write([]byte(err.Error()))
    }
    w.Write([]byte("ok"))
  }
}

func (s *webserver) handleNewConn(w http.ResponseWriter, r *http.Request) {
  vs := make(map[string]string)
  for _, k := range []string{"network", "rendezvous", "psk", "local"} {
    if v := r.PostFormValue(k); v == "" {
      w.WriteHeader(400)
      w.Write([]byte("Missing form item: " + k))
      return
    } else {
      vs[k] = v
    }
  }
  net := vs["network"]

  req := &pb.ConnectRequest{
    Network: net,
    Addr: get(r, "addr", "/ip4/0.0.0.0/tcp/0"),
    Rendezvous: vs["rendezvous"],
    Psk: vs["psk"],
    Local: vs["local"] == "true",
    DbPath: get(r, "db_path", net + ".sqlite3"),
    PrivkeyPath: get(r, "privkey_path", net + ".priv"),
    PubkeyPath: get(r, "pubkey_path", net + ".pub"),
    DisplayName: get(r, "display_name", "anonymous"),
    ConnectTimeout: 0,
    SyncPeriod: 60*5,
    MaxRecordsPerPeer: 100,
    MaxTrackedPeers: 100,
  }
  if _, err := s.d.Command.Connect(r.Context(), req); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  w.Write([]byte("ok"))
}

func (s *webserver) handleDeleteRegistry(w http.ResponseWriter, r *http.Request) {
  if u := r.PostFormValue("uuid"); u == "" {
      w.WriteHeader(400)
      w.Write([]byte("Missing uuid"))
      return
    } else if l := r.PostFormValue("local"); l == "" {
      w.WriteHeader(400)
      w.Write([]byte("Missing local"))
      return
    } else {
      req := &pb.StopAdvertisingRequest{Uuid: u, Local: l == "true"}
      if _, err := s.d.Command.StopAdvertising(r.Context(), req); err != nil {
        w.WriteHeader(500)
        w.Write([]byte(err.Error()))
      }
      w.Write([]byte("ok"))
  }
}


func (s *webserver) handleNewRegistry(w http.ResponseWriter, r *http.Request) {
  vs := make(map[string]string)
  for _, k := range []string{"name", "rendezvous", "local"} {
    if v := r.PostFormValue(k); v == "" {
      w.WriteHeader(400)
      w.Write([]byte("Missing form item: " + k))
      return
    } else {
      vs[k] = v
    }
  }
  tags := []string{}
  for _, s := range strings.Split(get(r, "tags", ""), ",") {
    tags = append(tags, strings.TrimSpace(s))
  }
  links := []string{}
  for _, s := range strings.Split(get(r, "links", ""), ",") {
    links = append(links, strings.TrimSpace(s))
  }
  cfg := &pb.NetworkConfig {
    Uuid: uuid.New().String(),
    Name: vs["name"],
    Description: get(r, "description", ""),
    Tags: tags,
    Links: links,
    Location: get(r, "location", "unknown"),
    Rendezvous: vs["rendezvous"],
    Creator: get(r, "creator", "anonymous"),
    Created: time.Now().Unix(),
  }
  if err := s.d.Command.ResolveRegistry(vs["local"] == "true").DB.UpsertConfig(cfg, []byte(""), storage.RegistryTable); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  w.Write([]byte("ok"))
}

func (s *webserver) handleNewPassword(w http.ResponseWriter, r *http.Request) {
  p := r.PostFormValue("password")
  if err := s.d.SetAdminPassAndSalt(p); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  w.Write([]byte("ok"))
}

func (s *webserver) handleRoot(w http.ResponseWriter, r *http.Request) {
  if r.Method != "GET" {
    w.WriteHeader(404)
    return
  }
  if r.URL.Path == "/" {
    s.handleIndex(w, r)
  } else {
    s.fsh.ServeHTTP(w, r)
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
