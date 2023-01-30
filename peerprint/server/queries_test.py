import unittest
import tempfile
from .queries import DBReader

class DBReaderTest(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.NamedTemporaryFile(delete=True)
        self.r = DBReader(tmp.name)
        cur = self.r.con.cursor()
        with open('pkg/storage/schema.sql','r') as f:
            cur.executescript(f.read())
        cur.close()
        self.addCleanup(tmp.close)

class TestDBReaderEmpty(DBReaderTest):
    def testRecordOps(self):
        self.assertEqual(self.r.getRecords(), [])
        self.assertEqual(self.r.countRecords(), 0)


class TestDBReaderWithData(DBReaderTest):

    def setUp(self):
        super().setUp()

        cur = self.r.con.cursor()
        recs = [(f"u{i}", f"a{i}", f"s{i}", "1,2,3", "loc", 123, 0, 0, 0, "sig") for i in range(10)]
        cur.executemany("INSERT INTO records (uuid, approver, signer, tags, manifest, created, num, den, gen, signature) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", recs)

        cmps = [(f"u{i}", f"c{i}", "state", f"s{i}", 1234, "sig") for i in range(5)]
        cur.executemany("INSERT INTO completions (uuid, completer, completer_state, signer, timestamp, signature) VALUES (?, ?, ?, ?, ?, ?)", cmps)
        cur.close()

    def testRecordOps(self):
        self.assertEqual(len(self.r.getRecords()), 10)
        self.assertEqual(self.r.countRecords(), 10)
    
"""

func TestComputePeerTrustNoCompletions(t *testing.T) {
  db := testingDB()
  if got, err := db.ComputePeerTrust("arandompeer"); err != nil || got > 0 {
    t.Errorf("Got %v, %v want 0, nil", got, err)
  }
}

func TestComputePeerTrustOnlyUntrusted(t *testing.T) {
  db := testingDB()
  for i := 0; i < 2; i++ {
    mustAddSC(db, "signer", "completer")
  }
  // comp/incomp=0, hearsay=2, max_hearsay=2 -> 0.66...
  if got, err := db.ComputePeerTrust("completer"); err != nil || got != 2.0/3.0 {
    t.Errorf("Got %v, %v want 0.66..., nil", got, err)
  }
}

func TestComputePeerTrustOnlyTrusted(t *testing.T) {
  db := testingDB()
  for i := 0; i < 5; i++ {
    mustAddSC(db, db.id, "completer")
  }
  mustAddSCT(db, "uuid1", db.id, "completer", false)
  mustAddSCT(db, "uuid2", db.id, "completer", false)
  // comp=5, incomp=2 -> 3
  if got, err := db.ComputePeerTrust("completer"); err != nil || got != 3 {
    t.Errorf("Got %v, %v want 3, nil", got, err)
  }
}

func mustWTrust(db *sqlite3, peer string, trust float64) {
  if _, err := db.db.Exec(`INSERT INTO "trust" (peer, worker_trust, timestamp) VALUES (?, ?, ?);`, peer, trust, time.Now().Unix()); err != nil {
    panic(err)
  }
}

func TestComputeRecordWorkabilityNoWorkers(t *testing.T) {
  db := testingDB()
  want := 1.0
  if got, err := db.ComputeRecordWorkability("foo"); err != nil || got != want {
    t.Errorf("Got %v, %v, want %v, nil", got, err, want)
  }
}

func TestComputeRecordWorkabilityLowWTrust(t *testing.T) {
  db := testingDB()
  mustAddSCT(db, "record", "completer1", "completer1", false)
  mustAddSCT(db, "record", "completer2", "completer2", false)
  mustWTrust(db, "completer1", 0.5)
  mustWTrust(db, "completer2", 0.1)
  want := 4/(math.Pow(4, 1.6))
  if got, err := db.ComputeRecordWorkability("record"); err != nil || got != want {
    t.Errorf("Got %v, %v, want %v, nil", got, err, want)
  }
}

func TestComputeRecordWorkabilityHighWTrust(t *testing.T) {
  db := testingDB()
  mustAddSCT(db, "record", "completer1", "completer1", false)
  mustAddSCT(db, "record", "completer2", "completer2", false)
  mustWTrust(db, "completer1", 1)
  mustWTrust(db, "completer2", 4)
  want := 4/(math.Pow(4, 6))
  if got, err := db.ComputeRecordWorkability("record"); err != nil || got != want {
    t.Errorf("Got %v, %v, want %v, nil", got, err, want)
  }
}
