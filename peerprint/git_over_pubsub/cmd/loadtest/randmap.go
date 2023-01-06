package main

import (
  "math/rand"
)

type randmap[K comparable, V any] struct {
  Container map[K]V
  keys []K
  keyindex map[K]int
  d V
}

func newRandMap[K comparable, V any](d V) *randmap[K,V] {
  return &randmap[K,V]{
    Container: make(map[K]V),
    keyindex: make(map[K]int),
    keys: []K{},
    d: d,
  }
}

func (m *randmap[K,V]) Set(k K, v V) {
  if _, ok := m.keyindex[k]; !ok {
    m.keys = append(m.keys, k)
    m.keyindex[k] = len(m.keys)-1
  }
  m.Container[k] = v
}
func (m *randmap[K,V]) Get(k K) V {
  return m.Container[k]
}
func (m *randmap[K,V]) Random() V {
  if len(m.keys) == 0 {
    return m.d
  }
  return m.Container[m.keys[rand.Intn(len(m.keys))]]
}
func (m *randmap[K,V]) Count() int {
  return len(m.keys)
}
func (m *randmap[K,V]) Unset(k K) {
  if i, ok := m.keyindex[k]; ok {
    delete(m.keyindex, k)
    last := (len(m.keys) -1) == i
    m.keys[i] = m.keys[len(m.keys)-1] // Copy last into index
    m.keys = m.keys[:len(m.keys)-1] // Pop last
    if !last {
        otherK := m.keys[i] 
        m.keyindex[otherK] = i
    }
    delete(m.Container, k)
  }
}
