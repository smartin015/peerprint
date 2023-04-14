package main

import (
  "fmt"
  "path/filepath"
  "github.com/smartin015/peerprint/p2pgit/pkg/driver"
  "github.com/smartin015/peerprint/p2pgit/pkg/registry"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/www"
  "flag"
  "context"
  "strings"
  "log"
  "os"
)

var (
  baseDirFlag = flag.String("baseDir", "", "Base directory path prepended to all -*Dir parameters")

  // Status server flags
  wwwFlag      = flag.String("www", "localhost:0", "Address for hosting status page - set empty to disable")
  wwwDirFlag = flag.String("wwwDir", "", "Path to WWW serving directory - leave empty to use bundled assets")
  wwwConfigFlag = flag.String("wwwCfg", "www.yaml", "Config path")
  wwwPasswordFlag = flag.String("wwwPassword", "", "Set the password for the status server, then exit.")

  // Registry server flags
  regDBWorldFlag = flag.String("regDBWorld", "registry_world.sqlite3", "Path to registry database (use :memory: for ephemeral, inmemory DB")
  regDBLocalFlag = flag.String("regDBLocal", "registry_local.sqlite3", "Path to registry database (use :memory: for ephemeral, inmemory DB")

  // Command server flags
  driverConfigFlag = flag.String("driverCfg", "driver.yaml", "Config path")
  connDirFlag = flag.String("connDir", ".", "Base directory for server connection files (configs, DBs etc)")
  addrFlag = flag.String("addr", "localhost:0", "Address for command service")
  certsDirFlag = flag.String("certsDir", ".", "Path to certificate directory; used to identify peers in gRPC communications between clients and server, and for HTTPS serving")
  serverCertFlag = flag.String("serverCert", "server.crt", "Filename for server certificate in certsDir; if one does not exist, a self-signed cert is created")
  serverKeyFlag = flag.String("serverKey", "server.key", "Filename for server private key in certsDir; if one does not exist, a self-signed cert is created")
  rootCertFlag = flag.String("rootCert", "rootCA.crt", "Filename for root certificate in certsDir; if one does not exist, a self-signed cert is created")
  cookieStoreKeyFlag = flag.String("cookieStoreKey", "nomnomcookies", "Key for encrypting cookie store")

  logger = log.New(os.Stderr, "", 0)
)

func exists(dir, path string) bool {
  _, err := os.Stat(filepath.Join(dir, path))
  return !os.IsNotExist(err)
}

func mustGenCerts(certsDir string) {
  ca, cab, cak, err := crypto.SelfSignedCACert("CA")
  if err != nil {
    panic(err)
  }
  if err := crypto.WritePEM(cab, cak, filepath.Join(certsDir, *rootCertFlag), filepath.Join(certsDir, "root.priv")); err != nil {
    panic(err)
  }
  _, sb, sk, err := crypto.CASignedCert(ca, cak, "Server")
  if err != nil {
    panic(err)
  }
  if err := crypto.WritePEM(sb, sk, filepath.Join(certsDir, *serverCertFlag), filepath.Join(certsDir, *serverKeyFlag)); err != nil {
    panic(err)
  }
  _, cb, ck, err := crypto.CASignedCert(ca, cak, "Client1")
  if err != nil {
    panic(err)
  }
  if err := crypto.WritePEM(cb, ck, filepath.Join(certsDir, "client1.crt"), filepath.Join(certsDir, "client1.key")); err != nil {
    panic(err)
  }
}

func main() {
  l := pplog.New("main", logger)
  flag.Parse()
  if *baseDirFlag == "" {
    l.Error("-baseDir must be specified")
    return
  } else {
    l.Info("Using base directory %s", *baseDirFlag)
  }

  if *wwwPasswordFlag != "" {
    cfgPath := filepath.Join(*baseDirFlag, *wwwConfigFlag)
    if err := www.WritePassword(*wwwPasswordFlag, cfgPath); err != nil {
      panic(err)
    }
    l.Info("Password successfully written to %s; exiting", cfgPath)
    return
  }
  ctx := context.Background()

  certsDir := filepath.Join(*baseDirFlag, *certsDirFlag)
  l.Info("Certs directory: %s", certsDir)
  eSrvCert := exists(certsDir, *serverCertFlag)
  eSrvKey := exists(certsDir, *serverKeyFlag)
  eRootCert := exists(certsDir, *rootCertFlag)
  if !(eSrvCert && eSrvKey && eRootCert) {
    if eSrvCert || eSrvKey || eRootCert {
      panic(fmt.Errorf("Missing file, one of: %s, %s, % in %s", *serverCertFlag, *serverKeyFlag, *rootCertFlag, certsDir))
    }
    l.Warning("No certs provided; generating self-signed certificates")
    mustGenCerts(certsDir)
    l.Info("Certs generated in %s", certsDir)
  }

  rlDir := filepath.Join(*baseDirFlag, *regDBLocalFlag)
  l.Info("Local registry DB: %s", rlDir)
  rLocal, err := registry.New(ctx, rlDir, true, pplog.New("local_registry", logger))
  if err != nil {
    panic(err)
  }
  rwDir := filepath.Join(*baseDirFlag, *regDBWorldFlag)
  l.Info("World registry DB: %s", rwDir)
  rWorld, err := registry.New(ctx, rwDir, false, pplog.New("global_registry", logger))
  if err != nil {
    panic(err)
  }

  d := driver.New(&driver.Opts{
    RPCAddr: *addrFlag,
    CertsDir: certsDir,
    ServerCert: *serverCertFlag,
    ServerKey: *serverKeyFlag,
    RootCert: *rootCertFlag,
    ConfigPath: filepath.Join(*baseDirFlag, *driverConfigFlag),
    ConnectionDir: filepath.Join(*baseDirFlag, *connDirFlag),
  }, rLocal, rWorld, pplog.New("driver", logger))


  if *wwwFlag != "" {
    liveDir := *wwwDirFlag
    if !strings.HasPrefix(liveDir, "/") {
      liveDir = filepath.Join(*baseDirFlag, *wwwDirFlag)
    }
    wsrv := www.New(pplog.New("www", logger), d, &www.Opts{
      LiveDir: liveDir,
      CookieStoreKey: []byte(*cookieStoreKeyFlag),
      ConfigPath: filepath.Join(*baseDirFlag, *wwwConfigFlag),
    })
    certPath := filepath.Join(*baseDirFlag, *certsDirFlag, *serverCertFlag)
    keyPath := filepath.Join(*baseDirFlag, *certsDirFlag, *serverKeyFlag)
    go wsrv.Serve(ctx, *wwwFlag, certPath, keyPath)
  }

  if err := d.Start(ctx, true); err != nil {
    panic(err)
  }
  select{}
}

