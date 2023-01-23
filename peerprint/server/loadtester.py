from .queue import P2PQueue
from .proc import ServerProcessOpts
from random import random
import peerprint.server.pkg.proto.state_pb2 as spb
import uuid
import logging
import tempfile
import time

logging.basicConfig(level=logging.DEBUG)
l = logging.getLogger()
def em(text):
  return "\u001b[35m" + text + "\u001b[0m"

BINPATH = "./server"

l.debug(em("Creating queue"))
opts = ServerProcessOpts(
    testQPS=0.5,
    testRecordTarget=10,
    rendezvous="testing",
    psk="12345",
    www="0.0.0.0:5000",
    local=True,
    displayName="Test server",
)
q = P2PQueue(opts, "TODO", BINPATH, "TODO", l.getChild("queue"))

l.debug(em("Connecting"))
q.connect()

l.debug(em("Waiting for initial update"))
q.waitForUpdate()

def addRecord():
    rep = q.set(spb.Record(
        uuid=uuid.uuid4(),
        location=uuid.uuid4(),
        created=int(time.time()),
        rank=spb.Rank(num=0, den=0, gen=0),
    ))
    ldebug(em(f"addRecord(): {rep}"))

def modRecord():
    l.debug(em("TODO modRecord"))

def rmRecord():
    r = q.reader.getRandomRecord()
    if r is None:
        return
    
    r.tombstone = int(time.time())
    rep = q.set(r)
    ldebug(em(f"rmRecord(): {rep}"))

def addCompletion():
    rep = q.set(spb.Completion(
        completer=q.get_id(),
        approver="todo",
        timestamp=int(time.time()),
        )
      )
    ldebug(em(f"addCompletion(): {rep}"))

l.debug(em(f"Looping - will make random changes with a mean of {opts.testQPS} qps"))
successes = 0
errors = {}
while True:
    time.sleep((1.0/opts.testQPS) + opts.testQPS*(0.2*random()-0.1))
    
    """
    nrec, _ := l.s.CountRecords()
    // We become less likely to add a record as we approach the target
    pAdd := float32(l.targetRecords-nrec)/float32(l.targetRecords)
    if rec == nil || rand.Float32() < pAdd {
    return "addRecord()", l.addRecord()
    }

    result = makeRandomChange()
    l.debug(f"TODO handle result {result}")



  // Precondition: we have a record and a server, but maybe not a completion.
  if rand.Float32() < 0.9 {
    if g == nil {
      return fmt.Sprintf("requestCompletion(%s, _)", l.srv.ShortID()), l.requestCompletion(rec.Record)
    } else {
      err := l.s.GetSignedRecord(g.Completion.Scope, rec)
      if rec == nil || err != nil {
        return "get record matching completion", derr(ErrNoRecord, "error %v", err)
      }
    }
    // Post: g completions srv access to rec; all entities exist
  }

  if p := rand.Float32(); p < 0.1 {
    return "rmRecord()", l.rmRecord(rec.Record)
  } else if p < 0.3 {
    return "requestCompletion()", l.requestCompletion(rec.Record)
  } else {
    return "mutateRecord()", l.mutateRecord(rec.Record)
  }
  */
  return "", nil
}
"""
