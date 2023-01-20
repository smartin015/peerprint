package server

type unionFind struct {
  m map[string]string
}

func NewUnionFind(root string) *unionFind {
  u := &unionFind{
    m: make(map[string]string),
  }
  u.m[root] = root
  return u
}

func (u *unionFind) Add(k string, target string) {
  if t, ok := u.m[k]; ok {
    if _, ok := u.m[target]; !ok {
      u.m[target] = target
    }
    u.union(t, target)
    //log.Printf("union(%s, %s)", t, target)
  } else {
    u.m[k] = target
    //log.Printf("add u.m[%s] = %s", k, target)
  }
}

func (u *unionFind) union(a, b string) {
  // Naive union, may cause tall trees but grants should be small so let's optimize later.
  at := u.Find(a)
  bt := u.Find(b)
  if at == bt {
    return
  } else {
    u.m[a] = b
  }
}

func (u *unionFind) Find(k string) string {
  // Naive, no path splitting
  for v, ok := u.m[k]; ok && v != k; v, ok = u.m[k] { // Trace until loop or lookup fail
    //log.Printf("Find(%s) -> %s, %v. Setting %s=%s\n", k, v, ok, k, v)
    k = v
  }
  return k
}
