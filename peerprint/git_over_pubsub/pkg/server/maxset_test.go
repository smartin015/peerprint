package server

import (
  "testing"
)

func TestSingle(t *testing.T) {
  s := NewMaxSet()
  s.Add("a", 0)
  if got := s.Get("a"); got != 0 {
    t.Errorf("Get(a) want 0 got %d", got)
  }
}

func TestNoIncr(t *testing.T) {
  s := NewMaxSet()
  s.Add("a", 100)
  s.Add("a", 99)
  s.Add("a", 98)
  if got := s.Get("a"); got != 100 {
    t.Errorf("Get(a) want 100 got %d", got)
  }
}

func TestIncr(t *testing.T) {
  s := NewMaxSet()
  s.Add("a", 98)
  s.Add("a", 99)
  s.Add("a", 100)
  if got := s.Get("a"); got != 100 {
    t.Errorf("Get(a) want 100 got %d", got)
  }
}
