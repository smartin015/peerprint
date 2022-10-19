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
        while self._log_sock is not None:
            msg = self._log_sock.recv().decode('utf8').rstrip()
            self._logger.info(msg)


class ZMQClient():
    def __init__(self, addr, logger):
        self._logger = logger
        self._context = zmq.Context()
        self._sock = self._context.socket(zmq.REQ)
        self._sock.connect(addr) # connect to bound REP socket in golang code
        self._logger.debug(f"ZMQClient connect to {addr}")
        self._msgclss = []
        for k,p in [kp for m in (spb, jpb, ppb) for kp in m.__dict__.items()]:
            if inspect.isclass(p) and issubclass(p, Message):
                self._msgclss.append(p)
        self._logger.debug(f"Loaded {len(self._msgclss)} message classes")
    
    def call(self, p):
        amsg = Any()
        amsg.Pack(p)
        self._sock.send(amsg.SerializeToString())

        apb = Any()
        data = self._sock.recv()
        apb.ParseFromString(data)
        for p in self._msgclss:
            if apb.Is(p.DESCRIPTOR):
                m = p()
                apb.Unpack(m)
                if p == spb.Error:
                    raise ZMQCallError(m.status)
                return m
        raise MessageUnpackError(f"Could not unpack message: {apb}")
