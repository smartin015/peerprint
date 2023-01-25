import sqlite3
import peerprint.server.pkg.proto.state_pb2 as spb

class DBReader:
    def __init__(self, dbPath):
        # Open in read-only mode; all writes must
        # occur via the server.
        dbPath = 'file:' + dbPath + '?mode=ro'
        print("Connecting to ", dbPath)
        self.con = sqlite3.connect(dbPath)
        self.con.execute('pragma journal_mode=wal')

    def _toRecord(self, vals):
        if vals is None:
            return None
        r = spb.SignedRecord()
        r.record.uuid = vals[0]
        for v in vals[1].split(","):
            r.record.tags.append(v)
        r.record.approver = vals[2]
        r.record.manifest = vals[3]
        r.record.created = vals[4]
        r.record.rank.num = vals[5]
        r.record.rank.den = vals[6]
        r.record.rank.gen = vals[7]
        r.signature.signer = vals[8]
        r.signature.data = vals[9]
        return r

    def _toCompletion(self, vals):
        if vals is None:
            return None
        c = spb.SignedCompletion()
        c.completion.uuid = vals[0]
        c.completion.completer = vals[1]
        c.completion.completer_state = vals[2]
        c.completion.timestamp = vals[3]
        c.signature.signer = vals[4]
        c.signature.data = vals[5]
        return c

    def _all(self, *args):
        cur = self.con.cursor()
        try:
            return cur.execute(*args).fetchall()
        finally:
            cur.close()

    def _one(self, *args):
        cur = self.con.cursor()
        try:
            return cur.execute(*args).fetchone()
        finally:
            cur.close()

    def _singleton(self, *args):
        try:
            return self._one(*args)[0]
        except IndexError:
            return None

    def countRecords(self):
        return self._singleton("SELECT COUNT(*) FROM records")

    def getRecords(self):
        return [self._toRecord(r) for r in self._all("SELECT * FROM records")]
