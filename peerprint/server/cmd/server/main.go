package main

import (
  "github.com/smartin015/peerprint/p2pgit/pkg/driver"
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

  addrFlag = flag.String("addr", "localhost:0", "Address for command service")
  certsDirFlag = flag.String("certsDir", "", "Path to certificate directory")
  serverCertFlag = flag.String("serverCert", "server_cert.pem", "Filename for server certificate in certsDir")
  serverKeyFlag = flag.String("serverKey", "server_key.pem", "Filename for server private key in certsDir")
  rootCertFlag = flag.String("rootCert", "ca_cert.pem", "Filename for root certificate in certsDir")

  logger = log.New(os.Stderr, "", 0)
)

func main() {
  flag.Parse()

  if *certsDirFlag == "" {
    panic("Require -certsDir")
  }
  
  d := driver.NewDriver(&driver.Opts{
    Addr: *addrFlag,
    CertsDir: *certsDirFlag,
    ServerCert: *serverCertFlag,
    ServerKey: *serverKeyFlag,
    RootCert: *rootCertFlag,
    ConfigPath: *configFlag,
  }, pplog.New("driver", logger))

  ctx := context.Background()

  if *wwwFlag != "" {
    wsrv := www.New(pplog.New("www", logger), d)
    go wsrv.Serve(*wwwFlag, ctx, *wwwDirFlag)
  }

  if err := d.Loop(ctx); err != nil {
    panic(err)
  }
}

