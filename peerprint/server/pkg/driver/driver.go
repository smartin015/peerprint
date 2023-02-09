package driver

import (
  "crypto/tls"
  "crypto/x509"
  "encoding/json"
  "path/filepath"
  //"google.golang.org/protobuf/proto"
  "google.golang.org/grpc"
  "google.golang.org/protobuf/proto"
  "google.golang.org/grpc/credentials"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/crawl"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  //"github.com/smartin015/peerprint/p2pgit/pkg/cmd"
  "strings"
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
  opts *Opts
  inst map[string]*Instance
  //cmdPush chan<- proto.Message
  config map[string]*pb.ConnectRequest
}

func NewDriver(opts *Opts, l *pplog.Sublog) *Driver {
  return &Driver{
    l: l,
    opts: opts,
    inst: make(map[string]*Instance),
    config: make(map[string]*pb.ConnectRequest),
  }
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
  for _, conf := range d.config {
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
  if _, err := os.Stat(d.opts.ConfigPath); !os.IsNotExist(err) {
    if err := d.readConfig(); err != nil {
      return fmt.Errorf("Read config: %w", err)
    }
  }

  /*
  d.l.Info("Initializing %d saved networks", len(d.config))
  for _, n := range d.config {
    if err := d.handleConnect(n); err != nil {
      d.l.Error(fmt.Errorf("Init %s: %w", n.Network, err))
    }
  }*/

  scp := filepath.Join(d.opts.CertsDir, d.opts.ServerCert)
  skp := filepath.Join(d.opts.CertsDir, d.opts.ServerKey)
  d.l.Info("Loading command server credentials - cert from %s, key from %s", scp, skp)
  cert, err := tls.LoadX509KeyPair(scp, skp)
  if err != nil {
    return fmt.Errorf("Load server cert: %w", err)
  }

  rcp := filepath.Join(d.opts.CertsDir, d.opts.RootCert)
  rcp_pem, err := os.ReadFile(rcp)
  if err != nil {
    return fmt.Errorf("read rcp file %w", err)
  }
  ccp := x509.NewCertPool()
  if ok := ccp.AppendCertsFromPEM(rcp_pem); !ok {
    return fmt.Errorf("failed to parse any root certificates from %s", rcp_pem)
  }

  creds := credentials.NewTLS(&tls.Config{
    Certificates: []tls.Certificate{cert},
    InsecureSkipVerify: true,
    RootCAs: ccp,
    ClientCAs: ccp,
    ClientAuth: tls.RequireAndVerifyClientCert,
  })
  grpcServer := grpc.NewServer(grpc.Creds(creds))
  pb.RegisterCommandServer(grpcServer, newCmdServer(d))
  lis, err := net.Listen("tcp", d.opts.Addr)
  if err != nil {
    return fmt.Errorf("Listen: %w", err)
  }
  d.l.Info("Command server listening on %s", d.opts.Addr)
  return grpcServer.Serve(lis)
}

func (d *Driver) readConfig() error {
  if len(d.config) > 0 {
    return fmt.Errorf("readConfig after init")
  }
  data, err := os.ReadFile(d.opts.ConfigPath)
  if err != nil {
    return err
  }
  for _, line := range strings.Split(string(data), "\n") {
    var v *pb.ConnectRequest
    if err := json.Unmarshal([]byte(line), v); err != nil {
      return err
    }
    d.config[v.Network] = v
  }
  return nil
}

func (d *Driver) writeConfig() error {
  f, err := os.Create(d.opts.ConfigPath)
  if err != nil {
    return err
  }
  defer f.Close()
  for _, n := range d.config {
    if data, err := json.Marshal(n); err != nil {
      f.Close()
      return err
    } else {
      f.Write(data)
      f.WriteString("\n")
    }
  }
  return nil
}

func (d *Driver) handleConnect(v *pb.ConnectRequest) error {
  if i, err := NewInstance(v, pplog.New(v.Network, d.l)); err != nil {
    return fmt.Errorf("Connect: %w", err)
  } else {
    d.inst[v.Network] = i
    // TODO use base context from driver
    go i.Run(context.Background())
    d.config[v.Network] = v
    d.writeConfig()
  }
  return nil
}

func (d *Driver) handleDisconnect(v *pb.DisconnectRequest) error {
  if i, ok := d.inst[v.Network]; !ok {
    return fmt.Errorf("Instance with rendezvous %q not found", v.Network) 
  } else {
    delete(d.config, v.Network)
    d.writeConfig()
    return i.Destroy()
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
