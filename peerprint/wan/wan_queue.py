import sys
import os
import time
import copy
import tempfile
from .comms import ZMQLogSink, ZMQClient
from .proc import ServerProcess
import peerprint.server.proto.state_pb2 as spb
import peerprint.server.proto.jobs_pb2 as jpb
import peerprint.server.proto.peers_pb2 as ppb
from google.protobuf.any_pb2 import Any
from enum import Enum

class ChangeType(Enum):
    JOBS = 0
    PEERS = 1

class PeerPrintQueue():
    def __init__(self, opts, peer_id, codec, binary_path, update_cb, logger, keydir=None):
        self._logger = logger
        self._opts = opts
        self._peer_id = peer_id
        self._binary_path = binary_path
        self._proc = None
        self._ready = False
        self._codec = codec
        self._update_cb = update_cb

        self._zmqlogger = None
        self._zmqclient = None
        self._proc = None

        # These are cached from updates
        self._jobs = dict()
        self._peers = ppb.PeersSummary(peer_estimate=0, variance=float('inf'), sample=[])
        # Using a temporary directory allows running multiple instances/queues
        # using the same filesystem (e.g. for development or containerized
        # farms)
        self.tmpdir = tempfile.TemporaryDirectory()
        if self._opts.zmqlog is None:
            self._opts.zmqlog = f"ipc://{self.tmpdir.name}/log.ipc"
        if self._opts.zmq is None:
            self._opts.zmq = f"ipc://{self.tmpdir.name}/cmd.ipc"
        if self._opts.zmqpush is None:
            self._opts.zmqpush = f"ipc://{self.tmpdir.name}/push.ipc"
        
        if keydir is not None:
            self._opts.privkeyfile = os.path.join(keydir, f"{opts.queue}_priv.key")
            self._opts.pubkeyfile = os.path.join(keydir, f"{opts.queue}_pub.key")

    def connect(self):
        self._zmqlogger = ZMQLogSink(self._opts.zmqlog, self._logger)
        self._proc = ServerProcess(self._opts, self._binary_path, self._logger)
        self._zmqclient = ZMQClient(self._opts.zmq, self._opts.zmqpush, self._update, self._logger)

    def destroy(self):
        if self._zmqlogger is not None:
            self._zmqlogger.destroy()
        if self._zmqclient is not None:
            self._zmqclient.destroy()
        if self._proc is not None:
            self._proc.destroy()
        self.tmpdir.cleanup()


    def _update(self, msg):
        expiry_ts = time.time() - 1*60*60 # TODO encode somewhere. manifest?
        if isinstance(msg, spb.State):
            newjobs = dict()
            for k,v in msg.jobs.items():
                newjobs[k] = self._codec.decode(v.data, v.protocol)
                newjobs[k]['peer_'] = v.owner
                if v.lock is not None and v.lock.created > expiry_ts:
                    newjobs[k]['acquired'] = True
                    newjobs[k]['acquired_by_'] = v.lock.peer
            # Set _jobs BEFORE calling update_cb in case it invokes get_jobs()
            oldjobs = copy.deepcopy(self._jobs)
            self._jobs = newjobs
            print("New job data:", self._jobs)
            if self._update_cb is not None:
                self._update_cb(ChangeType.JOBS, self._jobs, newjobs)
        elif isinstance(msg, ppb.PeersSummary):
            # Set _peers BEFORE calling update_cb in case it invokes get_peers()
            oldpeers = copy.deepcopy(self._peers)
            self._peers = msg
            if self._update_cb is not None:
                self._update_cb(ChangeType.PEERS, self._peers, msg)
        self._ready = True

    def is_ready(self):
        return self._ready

    def get_id(self):
        return self._peer_id

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

    def setJob(self, jid, manifest: dict, addr=None):
        data, protocol = self._codec.encode(manifest)
        self._zmqclient.call(jpb.SetJobRequest(
            job=jpb.Job(
                id=jid,
                owner=addr,
                protocol=protocol,
                data=data,
            ),
        ))

    def getJobs(self):
        return self._jobs

    def removeJob(self, jid: str):
        self._zmqclient.call(jpb.DeleteJobRequest(id=jid))

    def acquireJob(self, jid: str):
        self._zmqclient.call(jpb.AcquireJobRequest(id=jid))

    def releaseJob(self, jid: str):
        self._zmqclient.call(jpb.ReleaseJobRequest(id=jid))

