package cmd

import (
  "testing"
  "path/filepath"
  "gopkg.in/zeromq/goczmq.v4"
  "google.golang.org/protobuf/types/known/wrapperspb"
  "google.golang.org/protobuf/proto"
)

func makeIPC(t *testing.T) string {
  return "ipc://" + filepath.Join(t.TempDir(), "ipc")
}

type TestEnv struct {
  Recv <-chan proto.Message
  Err <-chan error
  Send chan<- proto.Message
  Push chan<- proto.Message

  ReqSock *goczmq.Sock
  PullSock *goczmq.Sock
}

func testEnv(t *testing.T) (*TestEnv) {
  rep_addr := makeIPC(t)
  push_addr := makeIPC(t)
  rc := make(chan proto.Message)
  ec := make(chan error)
  sc, pc, destroy := New(rep_addr, push_addr, rc, ec)
  t.Cleanup(func() {
    close(sc)
    close(rc)
    close(ec)
    destroy() // closes pc
  })

  req, err := goczmq.NewReq(rep_addr)
  if err != nil {
    panic(err)
  }
  t.Cleanup(req.Destroy)

  pull, err := goczmq.NewPull(push_addr)
  if err != nil {
    panic(err)
  }
  t.Cleanup(pull.Destroy)

  return &TestEnv{
    Recv: rc,
    Err: ec,
    Send: sc,
    Push: pc,
    ReqSock: req,
    PullSock: pull,
  }
}

func TestReceiveReply(t *testing.T) {
  want := &wrapperspb.StringValue{Value: "test1"}
  wantRep := &wrapperspb.StringValue{Value: "testrep"}
  e := testEnv(t)

  ser, err := Serialize(want)
  if err != nil {
    t.Fatal(err)
  }
  if err := e.ReqSock.SendMessage([][]byte{ser}); err != nil {
    t.Fatal(err)
  }

  got := <-e.Recv
  if !proto.Equal(got, want) {
    t.Errorf("receive: want %+v got %+v", want, got)
  }

  e.Send<- wantRep
  m, err := e.ReqSock.RecvMessage()
  if err != nil {
    t.Fatal(err)
  }
  got, err = Deserialize(m)
  if err != nil {
    t.Fatal(err)
  }

  if !proto.Equal(got, wantRep) {
    t.Errorf("reply: want %+v got %+v", want, got)
  }
}

func TestPush(t *testing.T) {
  e := testEnv(t)
  want := &wrapperspb.StringValue{Value: "test2"}
  // Send reply
  e.Push<- want

  m, err := e.PullSock.RecvMessage()
  if err != nil {
    t.Fatalf("RecvMessage: %v", err)
  }
  got, err := Deserialize(m)
  if err != nil {
    t.Fatalf("Deserialize: %v", err)
  }

  if !proto.Equal(got, want) {
    t.Errorf("cmd Push() failed to be read: want %+v got %+v", want, got)
  }
}

func TestLogger(t *testing.T) {
  ipc := makeIPC(t)
  l, ldest := NewLog(ipc)
  t.Cleanup(ldest)
  c, err := goczmq.NewPull(ipc)
  if err != nil {
    t.Fatal(err)
  }
  t.Cleanup(c.Destroy)

  s := "Test message"
  want := s + "\n"
  l.Println(s)
  msg, err := c.RecvMessage()
  if err != nil {
    t.Fatal(err)
  }
  got := string(msg[0])

  if got != want {
    t.Fatalf("l.Println(%q): want %q got %q", want, want, got)
  }
}
