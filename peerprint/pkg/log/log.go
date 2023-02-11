package log

import (
  "time"
  "fmt"
  "log"
  "os"
)

type withprintln interface {
  Println(...any)
  Fatal(...any)
}

type Sublog struct {
  n string
  l withprintln
}

func NewBasic() (*Sublog) {
  return &Sublog{
    n: "",
    l: log.New(os.Stderr, "", 0),
  }
}

func New(name string, l withprintln) (*Sublog) {
  return &Sublog{l: l, n: name}
}

func (l *Sublog) log(prefix string, args []interface{}, suffix string) {
  l.l.Println(fmt.Sprintf("%v %s(%s): ", time.Now().Format(time.RFC3339), prefix, l.n) + fmt.Sprintf(args[0].(string), args[1:]...) + suffix)
}
func (l *Sublog) Info(args ...interface{}) {
  l.log("\033[0mI", args, "\033[0m")
}
func (l *Sublog) Warning(args ...interface{}) {
  l.log("\033[35mW", args, "\033[0m")
}
func (l *Sublog) Error(args ...interface{}) {
  l.log("\033[33mE", args, "\033[0m")
}
func (l *Sublog) Fatal(args ...interface{}) {
  l.l.Fatal(args...)
}
func (l *Sublog) Println(v ...any) {
  v = append([]any{fmt.Sprintf("%s: ", l.n)}, v...)
  l.l.Println(v...)
}

