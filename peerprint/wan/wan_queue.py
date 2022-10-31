import sys
import os
import time
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
    def __init__(self, opts, codec, binary_path, update_cb, logger, keydir=None):
        self._logger = logger
        self._opts = opts
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
        # Including pid in sockets allows running multiple instances
        # using the same filesystem (e.g. for development or containerized
        # farms)
        pid = os.getpid()
        if self._opts.zmqlog is None:
            self._opts.zmqlog = f"ipc:///tmp/continuousprint_{opts.queue}_{pid}_log.ipc"
        if self._opts.zmq is None:
            self._opts.zmq = f"ipc:///tmp/continuousprint_{opts.queue}_{pid}.ipc"
        if self._opts.zmqpush is None:
            self._opts.zmqpush = f"ipc:///tmp/continuousprint_{opts.queue}_{pid}_push.ipc"
        
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


    def _update(self, msg):
        expiry_ts = time.time() - 1*60*60
        if isinstance(msg, spb.State):
            newjobs = dict()
            for k,v in msg.jobs.items():
                newjobs[k] = self._codec.decode(v.data, v.protocol)
                newjobs[k]['peer_'] = v.owner
                if v.lock is not None and v.lock.created > expiry_ts:
                    newjobs[k]['acquired'] = True
                    newjobs[k]['acquired_by_'] = v.lock.id
            self._update_cb(ChangeType.JOBS, self._jobs, newjobs)
            self._jobs = newjobs
        elif isinstance(msg, ppb.PeersSummary):
            self._update_cb(ChangeType.PEERS, self._peers, msg)
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

