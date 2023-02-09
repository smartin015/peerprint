import threading
import zmq
import inspect
import logging
import peerprint.server.pkg.proto.state_pb2 as spb
import peerprint.server.pkg.proto.peers_pb2 as ppb
import peerprint.server.pkg.proto.command_pb2 as cpb
from google.protobuf.message import Message
from google.protobuf.any_pb2 import Any

# print("Wrapper: ZMQ version is ", zmq.zmq_version())

class MessageUnpackError(Exception):
    pass
class CommandError(Exception):
    pass

class MutexSock():
    REQUEST_RETRIES = 3

    def __init__(self, ctx, typ, mut, logger, addr=None):
        self._logger = logger
        self._ctx = ctx
        self._addr = addr  # only needed for lazy pirate reconnection
        self._sock = ctx.socket(typ)
        self._mut = mut

    def bind(self, addr):
        self._sock.bind(addr)

    def connect(self, addr):
        self._sock.connect(addr)

    def recv(self, timeout=1):
        with self._mut:
            if self._sock.closed:
                return
            if (self._sock.poll(timeout*1000) & zmq.POLLIN) != 0:
                return self._sock.recv()
            else:
                return None

    def reqrep(self, req, timeout):
        # print("PY ->    LEN", len(req))
        with self._mut:
            self._sock.send(req)
            retries_left = self.REQUEST_RETRIES
            while True:
                if (self._sock.poll(timeout*1000/self.REQUEST_RETRIES) & zmq.POLLIN) != 0:
                    rep = self._sock.recv()
                    # print("   -> PY LEN", len(rep))
                    return rep
                retries_left -= 1
                # Socket is confused. Close and remove it.
                self._sock.close(linger=0)

                if retries_left == 0:
                    self._logger.error("Server seems to be offline")
                    raise zmq.error.ZMQError("Max retries exceeded")
                
                self._logger.warning("No response from server - reconnecting and resending")
                self._sock = self._ctx.socket(zmq.DEALER)
                self._sock.connect(self._addr)
                self._sock.send(req)

    def destroy(self):
        self._sock.close(linger=0)


class ZMQLogSink():
    # TODO probably not necessary
    def __init__(self, addr, mut, logger):
        self._logger = logger
        self._context = zmq.Context()
        self._log_sock = MutexSock(self._context, zmq.PULL, mut, self._logger)
        self._log_sock.bind(addr)
        self._log_thread = threading.Thread(target=self._stream_log, daemon=True)
        self._log_thread.start()
        self._logger.debug(f"ZMQLogSink thread started (bound to {addr})")
    
    def _stream_log(self):
        while self._log_sock is not None:
            try:
                msg = self._log_sock.recv(1.0)
                if msg is not None:
                    self._logger.info(msg.decode('utf8').rstrip())
            except zmq.error.ContextTerminated:
                return

    def destroy(self):
        self._log_sock.destroy()
        self._logger.debug("destroying log sock")
        self._context.destroy(linger=0)
        self._context = None
        self._log_sock = None

class ZMQClient():
    def __init__(self, req_addr, pull_addr, req_mut, pull_mut, cb, logger):
        self._logger = logger
        self._cb = cb
        self._context = zmq.Context()
        self._sock = MutexSock(self._context, zmq.DEALER, req_mut, self._logger, req_addr)
        self._sock.connect(req_addr) # connect to bound REP socket in golang code
        self._pull = MutexSock(self._context, zmq.PULL, pull_mut, self._logger)
        self._pull.bind(pull_addr) # bind for connecting PUSH socket in golang code
        self._logger.debug(f"ZMQClient connect to REQ {req_addr}")

        self._load_messages()
        self._pull_thread = threading.Thread(target=self._listen, daemon=True)
        self._pull_thread.start()
        self._logger.debug(f"ZMQClient listener thread started (bound to {pull_addr})")

    def _load_messages(self):
        self._msgclss = []
        for k,p in [kp for m in (spb, cpb, ppb) for kp in m.__dict__.items()]:
            if inspect.isclass(p) and issubclass(p, Message):
                self._msgclss.append(p)
        self._logger.debug(f"Loaded {len(self._msgclss)} message classes")
    
    def destroy(self):
        self._sock.destroy()
        self._logger.debug("destoyed comm socket")
        self._pull.destroy()
        self._logger.debug("destroyed pull socket")
        self._context.destroy(linger=0)
        self._context = None
        self._sock = None
        self._pull = None

    def _unpack(self, data):
        apb = Any()
        apb.ParseFromString(data)
        for p in self._msgclss:
            if apb.Is(p.DESCRIPTOR):
                m = p()
                apb.Unpack(m)
                if p == cpb.Error:
                    raise CommandError(m.reason)
                return m
        raise MessageUnpackError(f"Could not unpack message: {apb}")

    def _listen(self):
        while self._pull is not None:
            data = None
            try:
                data = self._pull.recv(1.0)
            except (zmq.error.ContextTerminated, zmq.error.ZMQError):
                return
            if data is not None:
                try:
                    self._cb(self._unpack(data))
                except MessageUnpackError as e:
                    self._logger.error(e)

    def call(self, p, timeout):
        amsg = Any()
        amsg.Pack(p)
        rep = self._sock.reqrep(amsg.SerializeToString(), timeout)
        return self._unpack(rep)
