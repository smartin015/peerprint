import sys
import signal
import json
import atexit
import subprocess
import threading
import zmq
import proto.state_pb2 as spb
import proto.jobs_pb2 as jpb
from google.protobuf.any_pb2 import Any
from enum import Enum

class ChangeType(Enum):
    PEER = "peer"
    JOB = "job"
    LOCK = "lock"
    QUEUE = "queue"

class PeerPrintQueue():
    def __init__(self, ns, update_cb, logger, local=True):
      self._logger = logger
      self._logger.debug("PeerPrintQueue init")
      self.ns = ns
      self._local = local
      self.sock_addr = f"ipc:///tmp/continuousprint_{ns}.ipc"
      self.log_addr = f"ipc:///tmp/continuousprint_{ns}_log.ipc"
      self.update_cb = update_cb 
      self._proc = None
      self._sock = None
    
    def _send(self, p):
        amsg = Any()
        amsg.Pack(p)
        self._sock.send(amsg.SerializeToString())

        apb = Any()
        data = self._sock.recv()
        apb.ParseFromString(data)
        for p in (spb.State, spb.Error):
            if apb.Is(p.DESCRIPTOR):
                m = p()
                apb.Unpack(m)
                return m
        raise Exception("Could not unpack message of type", apb.MessageName)

    def _handle(self, m):
        if isinstance(m, spb.State):
            print("Got state with", len(m.jobs), "Jobs:")
            for j in m.jobs.values():
                print(j.id)
        elif isinstance(m, spb.Error):
            print("ERROR:", m.status)

    def _tagged_cb(self, changetype):
        def tagcb(prev, nxt):
            # Unwrap metadata tuples and return end value (user provided)
            if prev is not None and type(prev) is tuple:
                prev = prev[-1]
            if nxt is not None and type(nxt) is tuple:
                nxt = nxt[-1]
            self.update_cb(changetype, prev, nxt)
        return tagcb
    
    def _stream_log(self):
        while self._log_sock is not None:
            msg = self._log_sock.recv().decode('utf8').rstrip()
            self._logger.info(msg)

    def connect(self):
        self._context = zmq.Context()
        args = ["../pubsub", "-registry=../example_registry.yaml",  "-bootstrap=true", f"-zmq={self.sock_addr}", f"-zmqlog={self.log_addr}"]
        if self._local:
            args.append("-local")

        # Bind and start logging thread before starting process so we don't miss any logs
        self._logger.info(f"Binding to ZMQ log addr {self.log_addr}")
        self._log_sock = self._context.socket(zmq.PULL)
        self._log_sock.bind(self.log_addr)
        self._log_thread = threading.Thread(target=self._stream_log, daemon=True)
        self._log_thread.start()

        self._proc = subprocess.Popen(args)
        atexit.register(self._cleanup)

        # Connect to command socket
        self._sock = self._context.socket(zmq.REQ)
        self._sock.connect(self.sock_addr) # connect to bound REP socket in golang code

    def _signal(self, sig, timeout=5):
        self._proc.send_signal(signal.SIGINT)
        try:
            self._proc.wait(timeout)
        except subprocess.TimeoutExpired:
            pass
        return self._proc.returncode is not None

    def _cleanup(self):
        if self._proc is None or self._proc.returncode is not None:
            return
        self._logger.info(f"Cleaning up peerprint process {self._proc.pid} (sending SIGINT)")
        if self._signal(signal.SIGINT):
            return

        self._logger.info(f"Escalating termination of process {self._proc.pid} (sending SIGKILL)")
        if self._signal(signal.SIGKILL):
            return

        self._logger.info(f"Escalating termination of process {self._proc.pid} (sending SIGTERM)")
        self._signal(signal.SIGKILL, timeout=None)

    def is_ready(self):
        return self._proc.returncode is None

    # ==== Mutation methods ====

    def syncPeer(self, state: dict, addr=None):
        if self._local:
            raise NotImplemented
        else:
            return # No peer data for non-local queues
    
    def getPeers(self):
        if self._local:
            raise NotImplemented
        else:
            return None # No peer data for non-local queues

    def getPeer(self, peer):
        if self._local:
            raise NotImplemented
        else:
            return None # No peer data for non-local queues

    def hasJob(self, jid) -> bool:
        return self.getJob(jid) is not None

    def setJob(self, jid, manifest: dict):
        rep = self._send(jpb.SetJobRequest(
            job=jpb.Job(
                id=jid,
                protocol="json",
                data=json.dumps(manifest).encode("utf8"),
            ),
        ))
        if isinstance(rep, spb.Error):
            raise Exception(rep.status)

    def getLocks(self):
        raise NotImplemented

    def getJobs(self):
        rep = self._send(jpb.GetJobsRequest())
        print("GetJobs reply:", rep)
        if isinstance(rep, spb.Error):
            raise Exception(rep.status)
        result = []
        for j in rep.jobs.values():
            if j.protocol == "json":
                try:
                    result.append(json.loads(j.data))
                except json.JSONDecodeError as e:
                    self._logger.error(f"JSON decode error for job {j.id}: {str(e)}")
            else:
                self._logger.error(f"No decoder for job {j.id} with protocol '{j.protocol}'")

        return result

    def getJob(self, jid):
        rep = self._send(jpb.GetJobsRequest())
        if isinstance(rep, spb.Error):
            raise Exception(rep.status)
        # TODO add lock data?
        for j in rep.jobs:
            if j.id == jid:
                return json.loads(j.data)

    def removeJob(self, jid: str):
        pass

    def acquireJob(self, jid: str):
        # rep = self._send(jpb.AcquireJobRequest(id=jid))
        pass

    def releaseJob(self, jid: str):
        pass


if __name__ == "__main__":
    import sys
    import logging
    import time
    import uuid
    assert(len(sys.argv) == 2)
    logging.basicConfig(level=logging.DEBUG)
    logger = logging.getLogger()

    def update_cb(typ, prev, nxt):
        print("update_cb", typ, prev, nxt)

    ns = sys.argv[1]
    q = PeerPrintQueue(ns, update_cb, logger, local=True)
    q.connect()
    while not q.is_ready():
        print("Waiting for ready state...")
        time.sleep(5)

    while True:
        v = input("Press Enter to query jobs, or type 'testjob' to insert a test job")
        if v == "testjob":
            print(q.setJob(str(uuid.uuid4()), dict(man="ifest")))
        else:
            try:
                print(q.getJobs())
            except Exception as e:
                print(str(e))


