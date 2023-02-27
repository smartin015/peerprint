package server

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "testing"
  "context"
  "sync"
  "fmt"
)

func TestGetSignedRecordsNone(t *testing.T) {
  s1 := NewTestServer(t, "rendy")
  req := make(chan struct{})
  defer close(req)
  rep := make(chan *pb.SignedRecord) // Closed by receiver
  srs, err := transport.Collect[*pb.SignedRecord](func() error {
    return s1.getService().GetSignedRecords(context.Background(), req, rep)
  }, rep)

  if err != nil {
    t.Errorf("GetSignedRecords error: %v", err)
  } else if len(srs) != 0 {
    t.Errorf("GetSignedRecords: want 0 records, got %v", srs)
  }
}

func TestGetSignedRecordsSome(t *testing.T) {
  s1 := NewTestServer(t, "rendy")
  s1.IssueRecord(&pb.Record{
    Uuid: "foo",
    Rank: &pb.Rank{},
  }, false)

  req := make(chan struct{})
  defer close(req)
  rep := make(chan *pb.SignedRecord) // Closed by receiver
  srs, err := transport.Collect[*pb.SignedRecord](func() error {
    return s1.getService().GetSignedRecords(context.Background(), req, rep)
  }, rep)

  if err != nil {
    t.Errorf("GetSignedRecords error: %v", err)
  } else if len(srs) != 1 {
    t.Errorf("GetSignedRecords: want 1 record, got %v", srs)
  }
}

func TestGetSignedCompletionsNone(t *testing.T) {
  s1 := NewTestServer(t, "rendy")
  req := make(chan struct{})
  defer close(req)
  rep := make(chan *pb.SignedCompletion) // Closed by receiver
  srs, err := transport.Collect[*pb.SignedCompletion](func() error {
    return s1.getService().GetSignedCompletions(context.Background(), req, rep)
  }, rep)

  if err != nil {
    t.Errorf("GetSignedCompletions error: %v", err)
  } else if len(srs) != 0 {
    t.Errorf("GetSignedCompletions: want 0 records, got %v", srs)
  }
}

func TestGetSignedCompletionsSome(t *testing.T) {
  s1 := NewTestServer(t, "rendy")
  s1.IssueCompletion(&pb.Completion{
    Uuid: "foo",
    CompleterState: []byte("test"),
  }, false)

  req := make(chan struct{})
  defer close(req)
  rep := make(chan *pb.SignedCompletion) // Closed by receiver
  srs, err := transport.Collect[*pb.SignedCompletion](func() error {
    return s1.getService().GetSignedCompletions(context.Background(), req, rep)
  }, rep)

  if err != nil {
    t.Errorf("GetSignedCompletions error: %v", err)
  } else if len(srs) != 1 {
    t.Errorf("GetSignedCompletions: want 1 record, got %v", srs)
  }
}

func TestGetPeers(t *testing.T) {
  if err := twoServerTest(t, "t1",
    func(s1, s2 *Server) error {return nil},
    func(s1, s2 *Server) error {
      rep := &pb.GetPeersResponse{}
      if err := s1.getService().GetPeers(context.Background(), &pb.GetPeersRequest{}, rep); err != nil {
        return err
      }
      if len(rep.Addresses) != 2 {
        return fmt.Errorf("Want addresses=2, got %v", rep.Addresses)
      }
      return nil
    },
  ); err != nil {
    t.Errorf(err.Error())
  }
}

func TestGetStatus(t *testing.T) {
  s1 := NewTestServer(t, "rendy")
  s1.SetStatus(&pb.PrinterStatus{
    Name: "testprinter",
  })
  rep := &pb.PeerStatus{}
  if err := s1.getService().GetStatus(context.Background(), &pb.GetStatusRequest{}, rep); err != nil {
    t.Errorf("GetStatus error: %v", err)
  }
  if len(rep.Printers) != 1 || rep.Printers[0].Name != "testprinter" {
    t.Errorf("Want PeerStatus with testprinter, got %v", rep)
  }
}
