package main

import (
  "math/rand"
  "github.com/google/uuid"
  "context"
  "time"
  "sync"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
)

type driver struct {
  servers *randmap[string,server.Interface]
  records *randmap[string,*pb.Record]
  grants *randmap[string,*pb.Grant]

  targetRecords int
  successes int
  errors map[ErrorType]int
}

func (d *driver) addRecord(srv server.Interface) *DriverErr {
  r := &pb.Record {
    Uuid: uuid.New().String(),
    Location: uuid.New().String(),
    Version: 0,
    Created: time.Now().Unix(),
    Rank: &pb.Rank{Num: 0, Den: 0, Gen: 0},
  }
  if err := srv.SetRecord(r); err != nil {
    return derr(ErrServer, "addRecord SetRecord error: %w", err)
  } else {
    d.records.Set(r.Uuid, r)
    return nil
  }
}

func (d *driver) rmRecord(srv server.Interface, rec *pb.Record) *DriverErr {
  rec.Tombstone = time.Now().Unix()
  if err := srv.SetRecord(rec); err != nil {
    return derr(ErrServer, "rmRecord SetRecord error: %w")
  } else {
    d.records.Unset(rec.Uuid)
    toRm := []string{}
    // Stop tracking grants for the removed record
    for k, g := range(d.grants.Container) {
      if g.Scope == rec.Uuid {
        toRm = append(toRm, k)
      }
    }
    for _, k := range(toRm) {
      d.grants.Unset(k)
    }
  }
  return nil
}

func (d *driver) requestGrant(srv server.Interface, rec *pb.Record) *DriverErr {
  g := &pb.Grant{
    Target: srv.ID(),
    Type: pb.GrantType_EDITOR,
    Scope: rec.Uuid,
  }
  if err := srv.SetGrant(g); err != nil {
    return derr(ErrServer, "requestGrant SetGrant error: %w", err)
  }
  d.grants.Set(rec.Uuid, g)
  return nil
}

func (d *driver) mutateRecord(srv server.Interface, rec *pb.Record) *DriverErr {
  rec.Version += 1
  rec.Location = uuid.New().String()
  if err := srv.SetRecord(rec); err != nil {
    return derr(ErrServer, "mutateRecord SetRecord error: %w", err)
  }
  return nil
}

func (d *driver) MakeRandomChange() (string, *DriverErr) {
  // Changes possible are 
  // - adding a new record
  // - acquiring (granting edit) an existing record
  // - removing (tombstoning) an existing record
  // - updating an existing record
  // 
  // Removals and updates require a grant already present; both present and non-present should be tested

  g := d.grants.Random()
  rec := d.records.Random()
  srv := d.servers.Random()
  if srv == nil {
    return "init", derr(ErrNoServer, "mutateRecord grant targets nonexistant server")
  }

  // We become less likely to add a record as we approach the target
  pAdd := float32(d.targetRecords-d.records.Count())/float32(d.targetRecords)
  if rec == nil || rand.Float32() < pAdd {
    return "addRecord", d.addRecord(srv)
  }

  // Precondition: we have a record and a server, but maybe not a grant.
  if rand.Float32() < 0.9 {
    if g == nil {
      return "requestGrant", d.requestGrant(srv, rec)
    } else {
      rec := d.records.Get(g.Scope)
      if rec == nil {
        return "get grant scope", derr(ErrNoRecord, "grant has nonexistant scope record")
      }
      srv = d.servers.Get(g.Target)
      if srv == nil {
        return "get grant target", derr(ErrNoServer, "grant has nonexistant target")
      }
    }
    // Post: g grants srv access to rec; all entities exist
  }

  if p := rand.Float32(); p < 0.1 {
    return "rmRecord", d.rmRecord(srv, rec)
  } else if p < 0.3 {
    return "requestGrant", d.requestGrant(srv, rec)
  } else {
    return "mutateRecord", d.mutateRecord(srv, rec)
  }
}

func NewDriver(servers []server.Interface, targetRecords int) *driver {
  d := &driver{
    successes: 0,
    errors: make(map[ErrorType]int),
    targetRecords: targetRecords,
    servers: newRandMap[string, server.Interface](nil),
    grants: newRandMap[string, *pb.Grant](nil),
    records: newRandMap[string, *pb.Record](nil),
  }
  for _,t := range ErrorTypes {
    d.errors[t] = 0
  }
  for _,s := range servers {
    d.servers.Set(s.ID(), s)
  }
  return d
}

func (d *driver) Run(duration time.Duration, qps float64) {
  dlog("Starting up servers with a deadline of %v and target record count of %d", duration, d.targetRecords)
  ctx, cancel := context.WithTimeout(context.Background(), duration)
  defer cancel()

  var wg sync.WaitGroup
  for _, s := range d.servers.Container {
    go s.Run(ctx)
    wg.Add(1)
    go func (srv server.Interface) {
      defer wg.Done()
      srv.WaitUntilReady()
      dlog("Server %s ready", srv.ID())
    }(s)
  }

  dlog("Waiting for servers to resolve initial state")
  wg.Wait()

  dlog("Making random changes at a rate of %f QPS", qps)
  ticker := time.NewTicker(time.Duration(int(1000.0/qps)) * time.Millisecond)
  
  for {
    select {
    case <-ticker.C:
      typ, err := d.MakeRandomChange()
      if err != nil {
        d.errors[err.Type] += 1
        dlog("%24s -> %s", typ, err.Error())
      } else {
        d.successes += 1
        dlog("%24s -> ok", typ)
      }
    case <-ctx.Done():
      dlog("Finishing run")
      return
    }
  }
}

func (d *driver) Verify() {
  tot := d.successes
  dlog("Successes: %d", d.successes)

  for typ, n := range(d.errors) {
    dlog("%s: %d", typ.String(), n)
    tot += n
  }
  dlog("Total: %d", tot)
  dlog("TODO verify all servers have consistent state")
}
