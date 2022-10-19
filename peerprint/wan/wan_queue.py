import sys
import time
import json
from .comms import ZMQLogSink, ZMQClient
from .proc import ServerProcess
import peerprint.server.proto.state_pb2 as spb
import peerprint.server.proto.jobs_pb2 as jpb
import peerprint.server.proto.peers_pb2 as ppb
from google.protobuf.any_pb2 import Any
from enum import Enum

class PeerPrintQueue():
    def __init__(self, opts, logger):
      self._logger = logger
      self._opts = opts
      self._proc = None
      self._ready = False
      # These are cached from updates
      self._jobs = dict()
      self._locks = dict()
      self._peers = ppb.PeersSummary(peer_estimate=0, variance=float('inf'), sample=[])
    
    def connect(self):
        self._zmqlogger = ZMQLogSink(self._opts.zmqlog, self._logger)
        self._proc = ServerProcess(self._opts, self._logger)
        self._zmqclient = ZMQClient(self._opts.zmq, self._opts.zmqpush, self._update, self._logger)

    def _parse(self, job):
        if job.protocol == "json":
            try:
                return json.loads(job.data)
            except json.JSONDecodeError as e:
                self._logger.error(f"JSON decode error for job {job.id}: {str(e)}")
        else:
            self._logger.error(f"No decoder for job {job.id} with protocol '{job.protocol}'")

    def _update(self, msg):
        expiry_ts = time.time() - 1*60*60
        if isinstance(msg, spb.State):
            newjobs = dict()
            newlocks = dict()
            for k,v in msg.jobs.items():
                newjobs[k] = self._parse(v)
                if v.lock is not None and v.lock.created > expiry_ts:
                    newlocks[k] = v.lock
            self._jobs = newjobs
            self._locks = newlocks
        elif isinstance(msg, ppb.PeersSummary):
            self._peers = msg
        self._ready = True

    def is_ready(self):
        return self._ready

    # ==== Mutation methods ====

    def syncPeer(self, state: dict, addr=None):
        self._zmqclient.call(ppb.PeerStatus()) #state=state, addr=addr))
    
    def getPeers(self):
        return self._peers

    def getPeer(self, peer):
        raise NotImplementedError()
        # return self._zmqclient.call(ppb.GetPeersRequest(peerFilter=peer))

    def hasJob(self, jid) -> bool:
        return self._jobs.get(jid) is not None

    def setJob(self, jid, manifest: dict):
        self._zmqclient.call(jpb.SetJobRequest(
            job=jpb.Job(
                id=jid,
                protocol="json",
                data=json.dumps(manifest).encode("utf8"),
            ),
        ))

    def getLocks(self):
        return self._locks

    def getJobs(self):
        return self._jobs

    def getJob(self, jid):
        return self._jobs.get(jid)

    def removeJob(self, jid: str):
        self._zmqclient.call(jpb.DeleteJobRequest(id=jid))

    def acquireJob(self, jid: str):
        self._zmqclient.call(jpb.AcquireJobRequest(id=jid))

    def releaseJob(self, jid: str):
        self._zmqclient.call(jpb.ReleaseJobRequest(id=jid))

