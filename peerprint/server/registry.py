import sqlite3
import uuid
from pathlib import Path
from dataclasses import dataclass
import peerprint.server.pkg.proto.peers_pb2 as ppb
from .proc import ProcessOptsBase, ServerProcess
import logging
import tempfile
import time

@dataclass
class RegistryOpts(ProcessOptsBase):
    addr: str = None
    rendezvous: str = None
    local: bool = None
    db: str = None
    connectTimeout: str = None
    syncPeriod: str = None
    lockfile: str = None

class RegistryReader:
    def __init__(self, dbPath, timeout):
        start = time.time()
        while True:
            if time.time() > start+timeout:
                raise Exception("timeout waiting for path " + dbPath)
            if Path(dbPath).exists():
                break
            time.sleep(0.1)

        dbPath = 'file:' + dbPath
        print("Connecting to ", dbPath)
        self.con = sqlite3.connect(dbPath)
        self.con.execute('pragma journal_mode=wal')

        while True:
            if time.time() > start+timeout:
                raise Exception("timeout waiting for DB tables")
            try: 
                self.con.execute("SELECT * FROM schemaversion LIMIT 1")
            except sqlite3.OperationalError:
                time.sleep(0.1)
                continue
            break

    def _toNetwork(self, vals):
        if vals is None:
            return None
        r = ppb.NetworkConfig()
        r.uuid = vals[0]
        r.name = vals[1]
        r.description = vals[2]
        for v in vals[3].split("\n"):
            r.tags.append(v)
        for v in vals[4].split("\n"):
            r.links.append(v)
        r.location = vals[5]
        r.rendezvous = vals[6]
        r.creator = vals[7]
        r.created = vals[8]
        sig = vals[9]
        # Skip stats UUID
        s = ppb.NetworkStats()
        s.population = vals[11]
        s.completions_last7days = vals[12]
        s.records = vals[13]
        s.idle_records = vals[14]
        s.avg_completion_time = vals[15]
        return ppb.Network(config=r, stats=s, signature=sig)

    def _all(self, *args):
        cur = self.con.cursor()
        try:
            return cur.execute(*args).fetchall()
        finally:
            cur.close()

    def getLobby(self):
        return [self._toNetwork(r) for r in self._all("SELECT N.*, S.* FROM lobby N LEFT JOIN stats S ON N.uuid=S.uuid")]

    def register(self, conf, sig):
        with self.con:
            return self.con.execute("INSERT OR REPLACE INTO registry (uuid, name, description, tags, links, location, rendezvous, creator, created, signature) VALUES (?,?,?,?,?,?,?,?,?,?);",  (
                    conf.uuid,
                    conf.name,
                    conf.description,
                    "\n".join(conf.tags),
                    "\n".join(conf.links),
                    conf.location, 
                    conf.rendezvous,
                    conf.creator,
                    conf.created,
                    sig
                ))


class P2PRegistry():
    def __init__(self, opts, binary_path, logger):
        self._logger = logger
        self._opts = opts
        self._binary_path = binary_path

        self._tmpdir = tempfile.TemporaryDirectory()
        if self._opts.db is None:
            self._opts.db = f"{self._tmpdir.name}/state.db"

    def connect(self, timeout):
        self._proc = ServerProcess(self._opts, self._binary_path, self._logger.getChild("proc"))
        self.db = RegistryReader(self._opts.db, timeout)

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    # No lockfile to allow for concurrent test
    reg = P2PRegistry(RegistryOpts(lockfile=""), "./registry", logging.getLogger())
    reg.connect(5.0)
    print("Registering test config")
    cur = reg.db.register(ppb.NetworkConfig(uuid=str(uuid.uuid4()), name="bar", description="baz"), "")
    print("Insert:", cur.rowcount, "rows")
    while True:
        time.sleep(5.0)
        print([n.config.uuid for n in reg.db.getLobby()])

