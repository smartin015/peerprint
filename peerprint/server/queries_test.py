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
    
    
