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
  "log"
  "os"
)

var (
  // Status server flags
  wwwFlag      = flag.String("www", "localhost:0", "Address for hosting status page - set empty to disable")
  wwwDirFlag = flag.String("wwwDir", "", "Path to WWW serving directory - leave empty to use bundled assets")
  wwwConfigFlag = flag.String("wwwCfg", "www.yaml", "Config path")

  // Registry server flags
  regDBWorldFlag = flag.String("regdbworld", "world_registry.sqlite3", "Path to registry database (use :memory: for ephemeral, inmemory DB")
  regDBLocalFlag = flag.String("regdblocal", "local_registry.sqlite3", "Path to registry database (use :memory: for ephemeral, inmemory DB")

  // Command server flags
  driverConfigFlag = flag.String("driverCfg", "driver.www", "Config path")
  addrFlag = flag.String("addr", "localhost:0", "Address for command service")
  certsDirFlag = flag.String("certsDir", "", "Path to certificate directory; used to identify peers in gRPC communications between clients and server, and for HTTPS serving")
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

func main() {
  flag.Parse()
  if *certsDirFlag == "" {
    panic("Require -certsDir")
  }
  ctx := context.Background()

  eSrvCert := exists(*certsDirFlag, *serverCertFlag)
  eSrvKey := exists(*certsDirFlag, *serverKeyFlag)
  eRootCert := exists(*certsDirFlag, *rootCertFlag)
  if !(eSrvCert && eSrvKey && eRootCert) {
    if eSrvCert || eSrvKey || eRootCert {
      panic(fmt.Errorf("Missing file, one of: %s, %s, %s in %s", *serverCertFlag, *serverKeyFlag, *rootCertFlag, *certsDirFlag))
    }
    logger.Printf("No certs provided; generating self-signed certificates\n")

    ca, cab, cak, err := crypto.SelfSignedCACert("CA")
    if err != nil {
      panic(err)
    }
    if err := crypto.WritePEM(cab, cak, filepath.Join(*certsDirFlag, *rootCertFlag), filepath.Join(*certsDirFlag, "root.priv")); err != nil {
      panic(err)
    }
    _, sb, sk, err := crypto.CASignedCert(ca, cak, "Server")
    if err != nil {
      panic(err)
    }
    if err := crypto.WritePEM(sb, sk, filepath.Join(*certsDirFlag, *serverCertFlag), filepath.Join(*certsDirFlag, *serverKeyFlag)); err != nil {
      panic(err)
    }
    _, cb, ck, err := crypto.CASignedCert(ca, cak, "Client1")
    if err != nil {
      panic(err)
    }
    if err := crypto.WritePEM(cb, ck, filepath.Join(*certsDirFlag, "client1.crt"), filepath.Join(*certsDirFlag, "client1.key")); err != nil {
      panic(err)
    }
    logger.Printf("Certs generated in dir %s", *certsDirFlag)
  }

  rLocal, err := registry.New(ctx, *regDBLocalFlag, true, pplog.New("local_registry", logger))
  if err != nil {
    panic(err)
  }

  rWorld, err := registry.New(ctx, *regDBWorldFlag, false, pplog.New("global_registry", logger))
  if err != nil {
    panic(err)
  }

  d := driver.New(&driver.Opts{
    RPCAddr: *addrFlag,
    CertsDir: *certsDirFlag,
    ServerCert: *serverCertFlag,
    ServerKey: *serverKeyFlag,
    RootCert: *rootCertFlag,
    ConfigPath: *driverConfigFlag,
  }, rLocal, rWorld, pplog.New("driver", logger))


  if *wwwFlag != "" {
    certPath := filepath.Join(*certsDirFlag, *serverCertFlag)
    keyPath := filepath.Join(*certsDirFlag, *serverKeyFlag)

    wsrv := www.New(pplog.New("www", logger), d, &www.Opts{
      LiveDir: *wwwDirFlag, 
      CookieStoreKey: []byte(*cookieStoreKeyFlag),
      ConfigPath: *wwwConfigFlag,
    })
    go wsrv.Serve(ctx, *wwwFlag, certPath, keyPath)
  }

  if err := d.Start(ctx, true); err != nil {
    panic(err)
  }
  select{}
}

