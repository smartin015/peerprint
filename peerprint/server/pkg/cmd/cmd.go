// package cmd implements process interface for python and other language processes to interact with peerprint
package cmd

import (
  "time"
  "os"
  "fmt"
  "log"
  "sync"
  "context"
  "path/filepath"
  "gopkg.in/zeromq/goczmq.v4"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
  // Import custom protos to add them to the global type registry, so anypb can resolve them when unmarshalling
	_ "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
)

const (
	MAX_RECV_RETRY = 10
	MAX_SEND_RETRY = 3
  DefaultTimeout = 10*time.Second
)

type HandlerFn func(context.Context, proto.Message) (proto.Message, error)
type Destructor func()

type Zmq struct {
  running bool
  wg sync.WaitGroup
  errChan chan<- error
  pushChan chan proto.Message
  handler HandlerFn
}

type Opts struct {
  RouterAddr string
  PushAddr string
  CertsDir string
  ServerCert string
}

func newOrGenCert(path string) *goczmq.Cert {
  if _, err := os.Stat(path); os.IsNotExist(err) {
    c := goczmq.NewCert()
    if err := c.Save(path); err != nil {
      panic(err)
    }
    return c
  }

 serverCert, err := goczmq.NewCertFromFile(path)
 if err != nil {
   panic(fmt.Errorf("Load server cert: %w", err))
 }
 return serverCert
}

func New(ctx context.Context, opts *Opts, handler HandlerFn, errChan chan<- error) (chan<- proto.Message, Destructor, error) {
  z := &Zmq {
    running: true,
    pushChan: make(chan proto.Message, 5),
    errChan: errChan,
    handler: handler,
 }
 z.wg.Add(2)
  
 serverCert := newOrGenCert(filepath.Join(opts.CertsDir, opts.ServerCert))

 go z.syncpipe(ctx, opts.RouterAddr, opts.CertsDir, serverCert)
 go z.pipe(z.pushChan, opts.PushAddr)
 return z.pushChan, z.Destroy, nil
}

// Returns a logger and its destructor
func NewLog(addr string) (*log.Logger, Destructor) {
  s, err := goczmq.NewPush(addr)
  if err != nil {
    panic(fmt.Errorf("create log pusher: %w", err))
  }
  rw, err := goczmq.NewReadWriter(s)
  if err != nil {
    panic(fmt.Errorf("create readwriter: %w", err))
  }
  return log.New(rw, "", 0), rw.Destroy
}

func (z *Zmq) Destroy() {
  z.running = false
  close(z.pushChan) // Closing pushChan breaks out of the pipe() loop for the PUSH socket
  z.wg.Wait()
}

func (z *Zmq) syncpipe(ctx context.Context, router_addr, cert_dir string, serverCert *goczmq.Cert) {
  defer z.wg.Done()
	var err error

  auth := goczmq.NewAuth()
  defer auth.Destroy()

  if err := auth.Curve(cert_dir); err != nil {
    // TODO move initialization into New() fn and return as errors instead of panics
    panic(fmt.Errorf("Set auth to curve mode: %w", err))
  }
  c := goczmq.NewSock(goczmq.Router)
  c.SetZapDomain("global")
  serverCert.Apply(c)
  c.SetCurveServer(1)
  if _, err := c.Bind(router_addr); err != nil {
    panic(fmt.Errorf("Bind: %w", err))
  }

  p, err := goczmq.NewPoller(c)
  if err != nil {
    panic(fmt.Errorf("Create poller: %w", err))
  }

  for z.running {
		var mm [][]byte

    var pollin *goczmq.Sock
    for {
      pollin = p.Wait(100)
      if !z.running {
        return
      }
      if pollin == c {
        break
      }
    }
		mm, err = c.RecvMessage()
    if err != nil {
      z.errChan <- fmt.Errorf("receiver error: %w", err)
      continue
    }

    if err != nil {
      z.errChan <- fmt.Errorf("Decrypt error: %w", err)
    } else if req, err := Deserialize(mm[2]); err != nil {
      // m[2] is 3rd (data) frame
      // https://zguide.zeromq.org/docs/chapter3/#The-DEALER-to-ROUTER-Combination
      z.errChan <- fmt.Errorf("deserialize error: %w", err)
    } else {
      // Handle the message
      go func () {
        ctx2, _ := context.WithTimeout(ctx, DefaultTimeout)
        rep, err := z.handler(ctx2, req)
        if err != nil {
          rep = &pb.Error{Reason: err.Error()}
        }
        nn, err := Serialize(rep)
        if err != nil {
          z.errChan <- fmt.Errorf("serialize error: %w", err)
          nn = []byte("") // balance req/rep
        }
        err = c.SendMessage([][]byte{mm[0], []byte(""), nn})
        if err != nil {
          z.errChan <- fmt.Errorf("ROUTER error: %w", err)
        }
      }()
    }
  }
}

func (z *Zmq) pipe(in <-chan proto.Message, push_addr string) {
  defer z.wg.Done()
  c, err := goczmq.NewPush(push_addr) // will Bind() by default
  if err != nil {
    panic(fmt.Errorf("Create PUSH socket: %w", err))
  }
  defer c.Destroy()
  for z.running {
    req, more := <-in
    if !more {
      return
    }
    data, err := Serialize(req)
    if err != nil {
      z.errChan <- err
    }

    retried := 0
    for ; retried < MAX_SEND_RETRY; retried++ {
      err := c.SendMessage([][]byte{data})
      if err == nil {
        break
      }
      z.errChan <- fmt.Errorf("Push error (%d retries): %w", retried, err)
    }
    if retried >= MAX_SEND_RETRY {
      z.errChan <- fmt.Errorf("closing and re-opening PUSH socket")
      // Recreate the socket
      c.SetLinger(0)
      c.Destroy()
      c, err = goczmq.NewPush(push_addr)
      if err != nil {
        panic(fmt.Errorf("Recreate PUSH socket: %w", err))
      }
    }
  }
}


func Deserialize(mm []byte) (proto.Message, error) {
  if len(mm) == 0 {
    return nil, fmt.Errorf("cannot deserialize empty msg")
  }

  any := anypb.Any{}
  if err := proto.Unmarshal(mm, &any); err != nil {
    return nil, fmt.Errorf("Failed to unmarshal ZMQ message to Any(): %w", err)
  }
  msg, err := any.UnmarshalNew()
  if err != nil {
    return nil, fmt.Errorf("Failed to unmarshal ZMQ message from Any() to final type: %w", err)
  }
  return msg, nil
}

func Serialize(req proto.Message) ([]byte, error) {
	any, err := anypb.New(req)
	if err != nil {
		return nil, fmt.Errorf("any-cast failed")
	}
	msg, err := proto.Marshal(any)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}

  // TODO sign message and return signature as well

  return msg, nil
}
