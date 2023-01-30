from .comms import CommandError
from .queue import P2PQueue
from .proc import ServerProcessOpts
from .queries import DBReader
from random import random
from zmq.error import ZMQError
import peerprint.server.pkg.proto.state_pb2 as spb
import peerprint.server.pkg.proto.command_pb2 as cpb
import uuid
import logging
import tempfile
import time
import sys

TEST_QPS = 0.5
TARGET_RECORDS = 5
TARGET_WORKLOAD = 2
REQUESTED_EXPIRY_SEC = 20
ACCEPTED_EXPIRY_SEC = 30
BINPATH = "./server"

class LoadTestDBReader(DBReader):
    def __init__(self, path, q):
        super().__init__(path)
        self.q = q

    def randomUnstartedOtherRecord(self):
        # Get another peer's record that hasn't been fully completed
        # so we can start on it.
        # Skip any records that we've already marked as having started
        return self._toRecord(self._one("""
            SELECT T1.* FROM records T1 
            LEFT JOIN completions T2 
            ON (T1.uuid=T2.uuid 
                AND (T2.completer=? OR T2.Timestamp>0))
            WHERE T1.approver != ? 
            AND T2.timestamp IS NULL
            ORDER BY RANDOM() LIMIT 1
        """, (self.q.get_id(),self.q.get_id())))

    def randomAssignedSelfCompletion(self):
        # Get another peer's in-progress completion for a record we own
        # so that we can mark it complete (or sponsor it)
        return self._toCompletion(self._one("""
            SELECT T1.* FROM completions T1
            LEFT JOIN records T2 
            ON T1.uuid=T2.uuid 
            WHERE T1.timestamp=0 
            AND T2.signer=T2.approver 
            AND T2.signer=?
            ORDER BY RANDOM() LIMIT 1
        """, (self.q.get_id(),)))


class LoadTester: 
    def __init__(self, logger):
        self._logger = logger

        self.debug("Creating queue")
        opts = ServerProcessOpts(
            rendezvous="testing",
            psk="12345",
            local=True,
            displayName="Test server",
	    syncPeriod="30s",
        )

        if len(sys.argv) > 1 and sys.argv[1] != "":
            opts.www=sys.argv[1] # e.g. 0.0.0.0:5000

        self.q = P2PQueue(opts, BINPATH, self._logger.getChild("queue"))

        self.debug("Connecting and waiting for response")
        self.q.connect()

        self.debug("Creating DB reader")
        self.db = LoadTestDBReader(opts.db, self.q)

        self.requested = dict()
        self.accepted = dict()

    def debug(self, text):
        self._logger.debug("\u001b[35m" + text + "\u001b[0m")
    def info(self, text):
        self._logger.info("\u001b[35m" + text + "\u001b[0m")

    def loop(self):
        self.debug(f"Looping - will make random changes with a mean of {TEST_QPS} qps")
        while True:
            time.sleep((1.0/TEST_QPS) + TEST_QPS*(0.2*random()-0.1))
            while len(self.requested) < TARGET_RECORDS:
                uuid = self.createRecord()
                if uuid is None:
                    break
                exp = (0.5+random()) * REQUESTED_EXPIRY_SEC
                self.requested[uuid] = time.time() + exp
                self.info(f"requested: {uuid} expiring in {exp}s")

            while len(self.accepted) < TARGET_WORKLOAD:
                uuid = self.acceptRandomRecord()
                if uuid is None:
                    break
                exp = (0.5+random()) * ACCEPTED_EXPIRY_SEC
                self.accepted[uuid] = time.time() + exp
                self.info(f"accepted: {uuid} expiring in {exp}s")

            self.manageRequested()
            self.manageAccepted()

    def createRecord(self):
        r = spb.Record(
            uuid=str(uuid.uuid4()),
            manifest=str(uuid.uuid4()),
            created=int(time.time()),
            approver=self.q.get_id(),
        )
        r.rank.num = 0
        r.rank.den = 0
        r.rank.gen = 0
        try: 
            self.q.set(r)
        except ZMQError:
            return None
        return r.uuid

    def acceptRandomRecord(self):
        r = self.db.randomUnstartedOtherRecord()
        if r is None:
            return None
        try: 
            self.q.set(spb.Completion(
                uuid=r.record.uuid,
                completer=self.q.get_id(),
                completer_state="loadtest".encode("utf8"),
                timestamp=0,
            ))
        except ZMQError:
            return None
        return r.record.uuid

    def manageRequested(self):
        """
        return self.q.set(spb.Completion(
            uuid=r.record.uuid,
            completer=self.q.get_id(),
            completer_state="loadtest".encode("utf8"),
            timestamp=int(time.time()), # Implies we've stopped working
        ))"""
        for uuid, exp in list(self.requested.items()):
            # If expired, give completion to someone or self-complete
            # and remove from tracking
            if time.time() > exp:
                self.info(f"Expiring request {uuid}")
                # TODO find an existing completer
                try: 
                    self.q.set(spb.Completion(
                        uuid=uuid,
                        completer=self.q.get_id(),
                        completer_state="loadtest".encode("utf8"),
                        timestamp=int(time.time()),
                    ))
                    del self.requested[uuid]
                except ZMQError:
                    self.info("ignoring ZMQError")

    def manageAccepted(self):
        """
        c = db.randomAssignedSelfCompletion()
        if c is None:
            return
        return self.q.set(spb.Completion(
            uuid=c.completion.uuid,
            completer=c.completion.completer,
            completer_state=c.completion.completer_state,
            timestamp=int(time.time()),
        ))"""
        for uuid, exp in list(self.accepted.items()):
            # If expired, self-complete and remove from tracking
            if time.time() > exp:
                self.info(f"Stopping work on {uuid}")
                try: 
                    self.q.set(spb.Completion(
                        uuid=uuid,
                        completer=self.q.get_id(),
                        completer_state="loadtest".encode("utf8"),
                        timestamp=int(time.time()),
                    ))
                    del self.accepted[uuid]
                except ZMQError:
                    self.info("ignoring ZMQError")



if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    lt = LoadTester(logging.getLogger("main"))
    lt.loop()

