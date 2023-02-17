package main

import (
  "github.com/smartin015/peerprint/p2pgit/pkg/driver"
  "github.com/smartin015/peerprint/p2pgit/pkg/registry"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/www"
  "flag"
  "context"
  "log"
  "os"
)

var (
  configFlag = flag.String("cfg", "config.json", "Config path")

  // Status server flags
  wwwFlag      = flag.String("www", "localhost:0", "Address for hosting status page - set empty to disable")
  wwwDirFlag = flag.String("wwwDir", "", "Path to WWW serving directory - leave empty to use bundled assets")

  // Registry server flags
  regDBWorldFlag = flag.String("regdbworld", "world_registry.sqlite3", "Path to registry database (use :memory: for ephemeral, inmemory DB")
  regDBLocalFlag = flag.String("regdblocal", "local_registry.sqlite3", "Path to registry database (use :memory: for ephemeral, inmemory DB")

  // Command server flags
  addrFlag = flag.String("addr", "localhost:0", "Address for command service")
  certsDirFlag = flag.String("certsDir", "", "Path to certificate directory")
  serverCertFlag = flag.String("serverCert", "server.crt", "Filename for server certificate in certsDir")
  serverKeyFlag = flag.String("serverKey", "server.key", "Filename for server private key in certsDir")
  rootCertFlag = flag.String("rootCert", "rootCA.crt", "Filename for root certificate in certsDir")

  logger = log.New(os.Stderr, "", 0)
)

func main() {
  flag.Parse()
  if *certsDirFlag == "" {
    panic("Require -certsDir")
  }
  ctx := context.Background()

  rLocal, err := registry.New(ctx, *regDBLocalFlag, true, pplog.New("local_registry", logger))
  if err != nil {
    panic(err)
  }

  rWorld, err := registry.New(ctx, *regDBWorldFlag, false, pplog.New("global_registry", logger))
  if err != nil {
    panic(err)
  }

  d := driver.New(&driver.Opts{
    Addr: *addrFlag,
    CertsDir: *certsDirFlag,
    ServerCert: *serverCertFlag,
    ServerKey: *serverKeyFlag,
    RootCert: *rootCertFlag,
    ConfigPath: *configFlag,
  }, rLocal, rWorld, pplog.New("driver", logger))


  if *wwwFlag != "" {
    wsrv := www.New(pplog.New("www", logger), d, *wwwDirFlag)
    go wsrv.Serve(*wwwFlag, ctx)
  }

  if err := d.Loop(ctx); err != nil {
    panic(err)
  }
}

