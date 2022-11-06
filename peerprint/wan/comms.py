import threading
import zmq
import inspect
import peerprint.server.proto.state_pb2 as spb
import peerprint.server.proto.peers_pb2 as ppb
import peerprint.server.proto.jobs_pb2 as jpb
from google.protobuf.message import Message
from google.protobuf.any_pb2 import Any

class MessageUnpackError(Exception):
    pass

class ZMQCallError(Exception):
    pass

class ZMQLogSink():
    def __init__(self, addr, logger):
        self._logger = logger
        self._context = zmq.Context()
        self._log_sock = self._context.socket(zmq.PULL)
        self._log_sock.bind(addr)
        self._log_thread = threading.Thread(target=self._stream_log, daemon=True)
        self._log_thread.start()
        self._logger.debug(f"ZMQLogSink thread started (bound to {addr})")
    
    def _stream_log(self):
        while self._log_sock is not None and not self._log_sock.closed:
            msg = self._log_sock.recv().decode('utf8').rstrip()
            self._logger.info(msg)

    def destroy(self):
        self._log_sock.destroy()
        self._log_sock = None


class ZMQClient():
    def __init__(self, req_addr, pull_addr, cb, logger):
        self._logger = logger
        self._cb = cb
        self._context = zmq.Context()
        self._sock = self._context.socket(zmq.REQ)
        self._pull = self._context.socket(zmq.PULL)
        self._sock.connect(req_addr) # connect to bound REP socket in golang code
        self._pull.bind(pull_addr) # bind for connecting PUSH socket in golang code
        self._logger.debug(f"ZMQClient connect to REQ {req_addr}")

        self._msgclss = []
        for k,p in [kp for m in (spb, jpb, ppb) for kp in m.__dict__.items()]:
            if inspect.isclass(p) and issubclass(p, Message):
                self._msgclss.append(p)
        self._logger.debug(f"Loaded {len(self._msgclss)} message classes")

        self._pull_thread = threading.Thread(target=self._listen, daemon=True)
        self._pull_thread.start()
        self._logger.debug(f"ZMQClient listener thread started (bound to {pull_addr})")
    
    def destroy(self):
        self._sock.destroy()
        self._pull.destroy()

    def _unpack(self, data):
        apb = Any()
        apb.ParseFromString(data)
        for p in self._msgclss:
            if apb.Is(p.DESCRIPTOR):
                m = p()
                apb.Unpack(m)
                if p == spb.Error:
                    raise ZMQCallError(m.status)
                return m
        raise MessageUnpackError(f"Could not unpack message: {apb}")

    def _listen(self):
        while self._pull is not None and not self._pull.closed:
            data = self._pull.recv()
            try:
                self._cb(self._unpack(data))
            except MessageUnpackError as e:
                self._cb(e)

    def call(self, p):
        amsg = Any()
        amsg.Pack(p)
        self._sock.send(amsg.SerializeToString())
        data = self._sock.recv()
        return self._unpack(data)
