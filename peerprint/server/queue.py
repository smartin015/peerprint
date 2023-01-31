import sys
import os
import time
import copy
import tempfile
from zmq.error import ZMQError
from threading import Lock, Thread
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
    HEALTHCHECK_PD = 5.0
    RECONNECT_TIMEOUT = 30.0

    def __init__(self, opts, binary_path, logger):
        self._logger = logger
        self._opts = opts
        self._binary_path = binary_path
        self.state = None
        self._proc = None
        self._id = None
        self._cond = Condition()
        self._do_healthcheck = False
        self._mut = [Lock() for l in range(3)] # Used for halting sockets during restart
        self._op_mut = Lock()

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
        if self._opts.privKeyPath is None:
            self._opts.privKeyPath = f"{self._tmpdir.name}/key.priv"
        if self._opts.pubKeyPath is None:
            self._opts.pubKeyPath = f"{self._tmpdir.name}/key.pub"

    def _healthcheck_loop(self):
        self._logger.debug("Starting healthcheck loop")
        while True:
            time.sleep(self.HEALTHCHECK_PD)
            try: 
                if self._do_healthcheck:
                    ret = self._call(cpb.HealthCheck(), 3.0)
            except ZMQError:
                # TODO exponential backoff to reduce churn
                self._logger.error("ZMQ timeout; restarting server")
                self._restart_server()

    def _restart_server(self):
        self._do_healthcheck = False
        with self._op_mut:
            # Wait for all sockets to not be sending/receiving before
            # destroying them
            if self._proc is not None:
                self._logger.debug("Destroying process")
                self._proc.destroy()
                self._proc = None

            with self._mut[0]:
                self._logger.debug("Lock acquired; destroying logger")
                if self._zmqLogger is not None:
                    self._zmqLogger.destroy()
                    self._zmqLogger = None

            with self._mut[1]:
                with self._mut[2]:
                    self._logger.debug("Locks acquired; destroying zmqClient")
                    if self._zmqclient is not None:
                        self._zmqclient.destroy()
                        self._zmqclient = None

            with self._cond:
                self._logger.debug("initializing logsink")
                self._zmqLogger = ZMQLogSink(self._opts.zmqLog, self._mut[0], self._logger.getChild("zmqlog"))
                self._logger.debug("initializing server process")
                self._proc = ServerProcess(self._opts, self._binary_path, self._logger.getChild("proc"))
                self._logger.debug("initializing zmq client")
                self._zmqclient = ZMQClient(self._opts.zmq, self._opts.zmqPush, self._mut[1], self._mut[2], self._update, self._logger.getChild("zmqclient"))
                self._do_healthcheck = True

                self._logger.debug("entering cond wait")
                self._cond.wait(self.RECONNECT_TIMEOUT)


    def connect(self, timeout=None):
        Thread(target=self._healthcheck_loop, daemon=True).start()
        self._restart_server()

        # Must initialize the reader *after* the server is initialized
        # otherwise the DB file may not exist
        self.state = DBReader(self._opts.db) 

    def waitForUpdate(self, timeout=None):
        with self._cond:
            self._cond.wait(timeout)

    def destroy(self):
        with self._op_mut:
            if self._zmqLogger is not None:
                self._zmqLogger.destroy()
            if self._zmqclient is not None:
                self._zmqclient.destroy()
            if self._proc is not None:
                self._proc.destroy()
            self.tmpdir.cleanup()

    def _update(self, msg):
        # self._logger.debug(f"Got update message: {msg}")
        with self._cond:
            self._cond.notify_all()

    def _call(self, v, timeout=15):
        with self._op_mut:
            rep = self._zmqclient.call(v, timeout)
        if isinstance(rep, cpb.Error):
            raise Exception(rep.Reason)
        else:
            return rep

    # ==== command methods ====

    def get_id(self):
        if self._id is None:
            self._id = self._call(cpb.GetID()).id
        return self._id

    def set(self, v):
        return self._call(v)

    def setWorkerTrust(self, peer, t):
        return self._call(cpb.SetWorkerTrust(
            peer=peer,
            trust=t,
        ))

    def setRewardTrust(self, peer, t):
        return self._call(cpb.SetRewardTrust(
            peer=peer,
            trust=t,
        ))
    
    def setWorkability(self, uuid, w):
        return self._call(cpb.SetWorkability(
            uuid=uuid,
            workability=w,
        ))

