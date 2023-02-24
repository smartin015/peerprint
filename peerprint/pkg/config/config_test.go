package config

import (
  "testing"
  "os"
  "path/filepath"
)

type testcfg struct {
  A bool
  B int
  C string
}

func TestReadWrite(t *testing.T) {
  td, err := os.MkdirTemp("", "")
  if err != nil {
    t.Fatal(err)
  }
  path := filepath.Join(td, "conf.yaml")
  c := &testcfg{
    A: true,
    B: 5,
    C: "hello",
  }
  if err := Write(c, path); err != nil {
    t.Errorf("Write(%s): %v", path, err)
  } else {
    d := &testcfg{}
    if err := Read(d, path); err != nil {
      t.Errorf("Read(%s): %v", path, err)
    } else if d.A != c.A || d.B != c.B || d.C != c.C {
      t.Errorf("got %v want %v", d, c)
    }
  }
}

