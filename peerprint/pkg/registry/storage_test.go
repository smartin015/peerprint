package registry

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "bytes"
  "testing"
)

func TestConfigUpsertDelete(t *testing.T) {
  r := testLocalRegistry(t)
  if err := r.UpsertConfig(&pb.NetworkConfig{
    Uuid: "foo",
    Creator: r.t.ID(),
  }, []byte(""), RegistryTable); err != nil {
    t.Errorf("Upsert error: %v", err)
  }
  nn := getReg(r, RegistryTable)
  if len(nn) != 1 {
    t.Errorf("Upsert didn't upsert: got %v", nn)
  }
  
  if err := r.DeleteConfig("foo", RegistryTable); err != nil {
    t.Errorf("DeleteConfig: %v", err)
  } else if nn = getReg(r, RegistryTable); len(nn) != 0 {
    t.Errorf("want empty registry, got %v", nn)
  }
}
func TestStatsUpsertGet(t *testing.T) {
  r := testLocalRegistry(t)
  if err := r.UpsertConfig(&pb.NetworkConfig{
    Uuid: "foo",
    Creator: r.t.ID(),
  }, []byte(""), RegistryTable); err != nil {
    t.Errorf("Upsert error: %v", err)
  } else if err := r.UpsertStats("foo", &pb.NetworkStats{
    Population: 5,
  }); err != nil {
    t.Errorf("Stats upsert error: %v", err)
  } else if nn := getReg(r, RegistryTable); len(nn) != 1 || nn[0].Stats.Population != 5 {
    t.Errorf("Want single registry with population 5: got %v", nn)
  }
}

func TestSignConfig(t *testing.T) {
  r := testLocalRegistry(t)
  if err := r.UpsertConfig(&pb.NetworkConfig{
    Uuid: "foo",
    Creator: r.t.ID(),
  }, []byte(""), RegistryTable); err != nil {
    t.Errorf("Upsert error: %v", err)
  } else if err := r.SignConfig("foo", []byte("1234")); err != nil {
    t.Errorf("Sign error: %v", err)
  } else if nn := getReg(r, RegistryTable); len(nn) != 1 || bytes.Compare(nn[0].Signature, []byte("1234")) != 0 {
    t.Errorf("Want single registry with signature 1234, got %v", nn)
  }
}
