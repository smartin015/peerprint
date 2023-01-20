package automation 

import (
  "fmt"
)

type ErrorType int
const (
  ErrServer ErrorType = iota
  ErrNoGrant
  ErrNoServer
  ErrNoRecord
)

var ErrorTypes = []ErrorType{
  ErrServer,
  ErrNoGrant,
  ErrNoServer,
  ErrNoRecord,
}
func (e ErrorType) String() string {
    return []string{
      "ErrServer",
      "ErrNoGrant",
      "ErrNoServer",
      "ErrNoRecord",
    }[e]
}

type LoadTestErr struct {
  fmt string
  Type ErrorType
  args []any
}
func derr(typ ErrorType, fmt string, args ...any) *LoadTestErr {
  return &LoadTestErr{
    fmt: fmt,
    Type: typ,
    args: args,
  }
}
func (f LoadTestErr) Error() string {
  str := fmt.Sprintf(f.fmt, f.args...)
  return fmt.Sprintf("driver error (type %s): %s", f.Type, str)
}
