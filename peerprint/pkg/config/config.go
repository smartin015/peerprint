package config

import (
  "gopkg.in/yaml.v3"
  "fmt"
  "os"
  "log"
)

func Write(cfg interface{}, dest string) error {
  data, err := yaml.Marshal(cfg)
  if err != nil {
    return fmt.Errorf("marshal YAML: %w", err)
  }
  log.Default().Printf("%s", data)

  f, err := os.Create(dest)
  if err != nil {
    return err
  }
  defer f.Close()
  f.Write(data)
  return nil
}

func Read[T any](cfg T, src string) error {
  data, err := os.ReadFile(src)
  if err != nil {
    return err
  }
  log.Default().Printf("%s", data)
  if err := yaml.Unmarshal(data, cfg); err != nil {
    return fmt.Errorf("parse YAML: %w", err)
  }
  return nil
}

