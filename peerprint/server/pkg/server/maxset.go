package server

type maxSet struct {
  s map[string]int64
}

func NewMaxSet() *maxSet {
  return &maxSet{
    s: make(map[string]int64),
  }
}

func (s *maxSet) Add(k string, v int64) {
  if v2, ok := s.s[k]; !ok || v > v2 {
    s.s[k] = v
  }
}

func (s *maxSet) Get(k string) int64 {
  return s.s[k]
}
