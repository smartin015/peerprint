package driver

import (
  "github.com/go-webauthn/webauthn/webauthn"
  //"google.golang.org/protobuf/proto"
  "google.golang.org/grpc"
  "google.golang.org/protobuf/proto"
  "google.golang.org/grpc/credentials"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/crawl"
  "github.com/smartin015/peerprint/p2pgit/pkg/config"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  "github.com/smartin015/peerprint/p2pgit/pkg/registry"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  //"github.com/smartin015/peerprint/p2pgit/pkg/cmd"
  "net"
  "context"
  "fmt"
  "os"
  "time"
  "errors"
)

const (
  MaxAddrPerPeer = 30
)

var (
  ErrContext = errors.New("Context canceled")
)

type Opts struct {
  Addr string
  CertsDir string
  ServerCert string
  ServerKey string
  RootCert string
  ConfigPath string
}


type Driver struct {
  l *pplog.Sublog
  Command *CommandServer
  RLocal *registry.Registry
  RWorld *registry.Registry
  opts *Opts
  inst map[string]*Instance
  //cmdPush chan<- proto.Message
  Config *config.DriverConfig
}

func New(opts *Opts, rLocal *registry.Registry, rWorld *registry.Registry, l *pplog.Sublog) *Driver {
  return &Driver{
    l: l,
    RLocal: rLocal,
    RWorld: rWorld,
    opts: opts,
    inst: make(map[string]*Instance),
    Config: config.New(),
  }
}

func (d *Driver) SetAdminPassAndSalt(p string) error {
  if p == "" {
    return errors.New("No password given")
  }
  if err := d.Config.SetPassword(p); err != nil {
    return err
  }
  return d.Config.Write(d.opts.ConfigPath)
}


func (d *Driver) RegisterCredentials(cred *webauthn.Credential) error {
  d.Config.Credentials = append(d.Config.Credentials, *cred)
  return d.Config.Write(d.opts.ConfigPath)
}

func (d *Driver) RemoveCredential(id string) error {
  for i, c := range d.Config.Credentials {
    if c.Descriptor().CredentialID.String() == id {
      d.Config.Credentials[i] = d.Config.Credentials[len(d.Config.Credentials)-1]
      d.Config.Credentials = d.Config.Credentials[:len(d.Config.Credentials)-1]
      return d.Config.Write(d.opts.ConfigPath)
    }
  }
  return fmt.Errorf("Credential %s not found", id)
}

func (d *Driver) InstanceNames() []string {
  nn := []string{}
  for k, _ := range d.inst {
    nn = append(nn, k)
  }
  return nn
}

func (d *Driver) GetConfigs(sanitized bool) []*pb.ConnectRequest {
  result := []*pb.ConnectRequest{}
  for _, conf := range d.Config.Connections {
    c2 := proto.Clone(conf).(*pb.ConnectRequest)
    if sanitized {
      c2.Psk = ""
    }
    result = append(result, c2)
  }
  return result
}

func (d *Driver) GetInstance(name string) *Instance {
  inst, ok := d.inst[name]
  if !ok {
    return nil
  }
  return inst
}

func (d *Driver) Loop(ctx context.Context) error {
  if _, err := os.Stat(d.opts.ConfigPath); os.IsNotExist(err) {
    d.l.Info("No config found - initializing with basic LAN queue")
    net := "LAN"
    d.Config.Connections[net] = &pb.ConnectRequest{
      Network: net,
      Addr: "/ip4/0.0.0.0/tcp/0",
      Rendezvous: net,
      Psk: net,
      Local: true,
      DbPath: net + ".sqlite3",
      PrivkeyPath: net + ".priv",
      PubkeyPath: net + ".pub",
      DisplayName: net,
      ConnectTimeout: 0,
      SyncPeriod: 60*5,
      MaxRecordsPerPeer: 100,
      MaxTrackedPeers: 100,
    }
    if err := d.Config.Write(d.opts.ConfigPath); err != nil {
      return err
    }
  } else {
    if err := d.Config.Read(d.opts.ConfigPath); err != nil {
      return fmt.Errorf("Read config: %w", err)
    }
  }
  d.l.Info("Config loaded - %d WebAuthn credential(s)", len(d.Config.Credentials))

  d.l.Info("Initializing %d network(s)", len(d.Config.Connections))
  for _, n := range d.Config.Connections {
    if err := d.handleConnect(n); err != nil {
      d.l.Error("Init %s: %v", n.Network, err)
    }
  }
	tlsCfg, err := crypto.NewTLSConfig(
		d.opts.CertsDir,
		d.opts.RootCert,
		d.opts.ServerCert,
		d.opts.ServerKey,
	)
	if err != nil {
		return fmt.Errorf("Construct TLS config: %w", err)
	}
  creds := credentials.NewTLS(tlsCfg)
  grpcServer := grpc.NewServer(grpc.Creds(creds))
  d.Command = newCmdServer(d)
  pb.RegisterCommandServer(grpcServer, d.Command)
  lis, err := net.Listen("tcp", d.opts.Addr)
  if err != nil {
    return fmt.Errorf("Listen: %w", err)
  }
  d.l.Info("Command server listening on %s", d.opts.Addr)
  return grpcServer.Serve(lis)
}


func (d *Driver) handleConnect(v *pb.ConnectRequest) error {
  if i, err := NewInstance(v, pplog.New(v.Network, d.l)); err != nil {
    return fmt.Errorf("Connect: %w", err)
  } else {
    d.inst[v.Network] = i
    // TODO use base context from driver
    go i.Run(context.Background())
    d.Config.Connections[v.Network] = v
    d.Config.Write(d.opts.ConfigPath)
  }
  return nil
}

func (d *Driver) handleDisconnect(v *pb.DisconnectRequest) error {
  if i, ok := d.inst[v.Network]; !ok {
    return fmt.Errorf("Instance with rendezvous %q not found", v.Network) 
  } else {
    delete(d.Config.Connections, v.Network)
    d.Config.Write(d.opts.ConfigPath)
    i.Destroy()
    return nil
  }
}

func (d *Driver) handleCrawl(ctx context.Context, inst *Instance, req *pb.CrawlRequest) (*pb.CrawlResult, error) {
  if req.RestartCrawl || inst.c == nil {
    inst.c = crawl.NewCrawler(inst.t.GetPeerAddresses(), inst.crawlPeer)
  }
  ctx2, _ := context.WithTimeout(ctx, time.Duration(req.TimeoutMillis) * time.Millisecond)
  remaining, errs := inst.c.Step(ctx2, req.BatchSize)

  errStrs := []string{}
  for _, err := range errs {
    errStrs = append(errStrs, err.Error())
  }
  return &pb.CrawlResult{
    Network: req.Network,
    Remaining: int32(remaining),
    Errors: errStrs,
  }, nil
}
