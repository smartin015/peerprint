import sys
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
    
    def connect(self):
        self._zmqlogger = ZMQLogSink(self._opts.zmqlog, self._logger)
        self._proc = ServerProcess(self._opts, self._logger)
        self._zmqclient = ZMQClient(self._opts.zmq, self._logger)

    def is_ready(self):
        print("Sending self status request")
        rep = self._zmqclient.call(ppb.SelfStatusRequest())
        print(rep)
        return rep.topic != ""

    # ==== Mutation methods ====

    def syncPeer(self, state: dict, addr=None):
        self._zmqclient.call(ppb.PeerStatus()) #state=state, addr=addr))
    
    def getPeers(self):
        return self._zmqclient.call(ppb.GetPeers())

    def getPeer(self, peer):
        return self._zmqclient.call(ppb.GetPeers(peerFilter=peer))

    def hasJob(self, jid) -> bool:
        return self.getJob(jid) is not None

    def setJob(self, jid, manifest: dict):
        self._zmqclient.call(jpb.SetJobRequest(
            job=jpb.Job(
                id=jid,
                protocol="json",
                data=json.dumps(manifest).encode("utf8"),
            ),
        ))

    def getLocks(self):
        rep = self._zmqclient.call(jpb.GetJobsRequest())
        print("GetJobs reply:", rep)
        result = []
        for j in rep.jobs.values():
            print(j.lock)

    def getJobs(self):
        rep = self._zmqclient.call(jpb.GetJobsRequest())
        print("GetJobs reply:", rep)
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
        rep = self._zmqclient.call(jpb.GetJobsRequest())
        # TODO add lock data?
        for j in rep.jobs:
            if j.id == jid:
                return json.loads(j.data)

    def removeJob(self, jid: str):
        self._zmqclient.call(jqb.DeleteJobsRequest(id=[jid]))

    def acquireJob(self, jid: str):
        self._zmqclient.call(jpb.AcquireJobRequest(id=jid))

    def releaseJob(self, jid: str):
        self._zmqclient.call(jqb.ReleaseJobRequest(id=jid))

