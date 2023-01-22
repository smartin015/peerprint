import sys
import os
import time
import copy
import tempfile
from .comms import ZMQLogSink, ZMQClient, MessageUnpackError
from .proc import ServerProcess
from .queries import DBReader
from multiprocessing import Condition
import peerprint.server.pkg.proto.state_pb2 as spb
import peerprint.server.pkg.proto.peers_pb2 as ppb
import peerprint.server.pkg.proto.command_pb2 as cpb
from google.protobuf.any_pb2 import Any
from enum import Enum

class ChangeType(Enum):
    JOBS = 0
    PEERS = 1

class P2PQueue():
    def __init__(self, opts, codec, binary_path, update_cb, logger):
        self._logger = logger
        self._opts = opts
        self._binary_path = binary_path
        self.state = None
        self._proc = None
        self._ready = False
        self._codec = codec
        self._cond = Condition()
        self._update_cb = update_cb

        self._zmqLogger = None
        self._zmqclient = None
        self._proc = None

        # Using a temporary directory allows running multiple instances/queues
        # using the same filesystem (e.g. for development or containerized
        # farms)
        self._tmpdir = tempfile.TemporaryDirectory()
        if self._opts.zmqLog is None:
            self._opts.zmqLog = f"ipc://{self._tmpdir.name}/log.ipc"
        if self._opts.zmq is None:
            self._opts.zmq = f"ipc://{self._tmpdir.name}/cmd.ipc"
        if self._opts.zmqPush is None:
            self._opts.zmqPush = f"ipc://{self._tmpdir.name}/push.ipc"
        if self._opts.db is None:
            self._opts.db = f"{self._tmpdir.name}/state.db"

    def connect(self):
        self._zmqLogger = ZMQLogSink(self._opts.zmqLog, self._logger.getChild("zmqlog"))
        self._proc = ServerProcess(self._opts, self._binary_path, self._logger.getChild("proc"))
        self.state = DBReader(self._opts.db)
        self._zmqclient = ZMQClient(self._opts.zmq, self._opts.zmqPush, self._update, self._logger.getChild("zmqclient"))

    def waitForUpdate(self, timeout=None):
        with self._cond:
            self._cond.wait(timeout)

    def destroy(self):
        if self._zmqLogger is not None:
            self._zmqLogger.destroy()
        if self._zmqclient is not None:
            self._zmqclient.destroy()
        if self._proc is not None:
            self._proc.destroy()
        self.tmpdir.cleanup()

    def _update(self, msg):
        if isinstance(msg, MessageUnpackError):
            self._logger.error(msg)
            return
        self._logger.debug(f"Got update message: {msg}")
        with self._cond:
            self._cond.notify_all()
        self._ready = True

    def is_ready(self):
        return self._ready

    def get_id(self):
        return self._opts.id

    # ==== Mutation methods ====
    
    def set(self, v):
        rep = self._zmqclient.call(v)
        if isinstance(rep, MessageUnpackError):
            raise rep
        elif isinstance(rep, cpb.Error):
            raise Exception(rep.Reason)
        else:
            return rep

