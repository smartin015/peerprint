package server

import (
  "testing"
)

func TestRootOnly(t *testing.T) {
  want := "root"
  u := NewUnionFind(want)
  if got := u.Find(want); got != want {
    t.Errorf("Find(%s) want %s got %s", want, want, got)
  }
}

func TestDirectNotRoot(t *testing.T) {
  want := "b"
  u := NewUnionFind("root")
  u.Add("a", want)
  if got := u.Find("a"); got != want {
    t.Errorf("Find(a) want %s got %s", want, got)
  }
}

func TestFindDanglingTarget(t *testing.T) {
  want := "b"
  u := NewUnionFind("root")
  u.Add("a", want)
  if got := u.Find(want); got != want {
    t.Errorf("Find(%s) want %s got %s", want, want, got)
  }
}

func TestDirectRoot(t *testing.T) {
  root := "root"
  u := NewUnionFind(root)
  u.Add("a", root)
  if got := u.Find("a"); got != root {
    t.Errorf("Find(a) want root got %s", got)
  }
}

func TestMultiSet(t *testing.T) {
  u := NewUnionFind("root")
  u.Add("a", "b")
  u.Add("a", "c")

  ga := u.Find("a")
  gb := u.Find("b")
  gc := u.Find("c")
  if ga != gb || gb != gc || gc != ga {
    t.Errorf("Want a=b=c, got a=%s, b=%s, c=%s", ga, gb, gc)
  }
}

func TestIndirectRoot(t *testing.T) {
  nodes := []string{"root", "a", "b", "c", "d", "e", "f", "g"}

  for i := 1; i < len(nodes); i += 1 {
    u := NewUnionFind(nodes[0])
    for j := 1; j <= i; j += 1 {
      u.Add(nodes[j], nodes[j-1])
    }
    if got := u.Find(nodes[i]); got != nodes[0] {
      t.Errorf("Want Find(%s) == %s, got %s", nodes[i], nodes[0], got)
    }
  }
}

func TestOverwriteIndirect(t *testing.T) {
  u := NewUnionFind("root")
  u.Add("a", "root")
  u.Add("b", "a")
  u.Add("a", "othernode")

  g1 := u.Find("a")
  g2 := u.Find("root")
  if g1 != g2 {
    t.Errorf("Find(a) vs Find(root): want equal, got %s vs %s", g1, g2)
  }
}
