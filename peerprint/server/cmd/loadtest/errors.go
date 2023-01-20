package main

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

type DriverErr struct {
  fmt string
  Type ErrorType
  args []any
}
func derr(typ ErrorType, fmt string, args ...any) *DriverErr {
  return &DriverErr{
    fmt: fmt,
    Type: typ,
    args: args,
  }
}
func (f DriverErr) Error() string {
  str := fmt.Sprintf(f.fmt, f.args...)
  return fmt.Sprintf("driver error (type %s): %s", f.Type, str)
}
