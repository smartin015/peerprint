package main

import (
  "testing"
  "math"
)

func TestUniformRandom(t *testing.T) {
  m := newRandMap[string, int](0)
  m.Set("1", 1)
  m.Set("2", 2)

  gots := make(map[int]int)
  N := 1000
  gots[1]=0
  gots[2]=0
  for i := 0; i < N; i++ {
    gots[m.Random()] += 1
  }
  d := math.Abs(float64(gots[1] - gots[2]) )
  if (d > 0.1*float64(N)) {
    t.Errorf("'Random' draw %f varies by more than 10pct of N (%d)", d, N)

  }
}
