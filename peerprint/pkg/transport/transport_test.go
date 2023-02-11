package transport

import (
  "testing"
  "google.golang.org/protobuf/types/known/wrapperspb"
	"github.com/libp2p/go-libp2p/core/crypto"
	libp2p "github.com/libp2p/go-libp2p"
  "crypto/rand"
)

func TestSignVerify(t *testing.T) {
  privk, _, err := crypto.GenerateEd25519Key(rand.Reader)
  if err != nil {
    t.Fatal(err.Error())
  }
  pid := libp2p.Identity(privk)
	h, err := libp2p.New(pid)
	if err != nil {
    t.Fatal(err.Error())
	}

  tr := &Transport{
    opts: &Opts{
      PrivKey: privk,
    },
    host: h,
  }
  m := &wrapperspb.StringValue{Value: "test1"}
  sig, err := tr.Sign(m) 
  if err != nil {
    t.Errorf("sign: %v", err)
  }

  valid, err := tr.Verify(m, tr.ID(), sig)
  if err != nil || !valid {
    t.Errorf("Verify = %v, %v; want true, nil", valid, err)
  }
}
