import unittest
import queue
from unittest.mock import MagicMock, patch
from .comms import ZMQLogSink, ZMQCallError, ZMQClient, MessageUnpackError
import logging
import zmq
from google.protobuf.wrappers_pb2 import StringValue
from google.protobuf.message import Message
from google.protobuf.any_pb2 import Any
import peerprint.server.proto.state_pb2 as spb

logging.basicConfig(level=logging.DEBUG)


class TestZMQLogSink(unittest.TestCase):
    def testStreamLog(self):
        class DummySock:
            def __init__(self):
                self.closed = False

            def bind(self, addr):
                pass

            def recv(self):
                self.closed = True
                return "test\n".encode('utf8')

            def closed(self):
                return self.closed

        with patch('peerprint.wan.comms.zmq') as z:
            log = MagicMock()
            z.Context().socket.return_value = DummySock()
            l = ZMQLogSink("testaddr", log)
            log.info.assert_called_once_with("test")

    def testDestroy(self):
        with patch('peerprint.wan.comms.zmq') as z:
            self.l = ZMQLogSink("testaddr", MagicMock())
            self.l.destroy()
            z.Context().socket(zmq.PULL).destroy.assert_called_once()

class TestZMQClient(unittest.TestCase):
    def setUp(self):
        class DummySock:
            def __init__(self):
                self.closed = False
                self.q = queue.Queue()
                self.connect = MagicMock()
                self.bind = MagicMock()
                self.send = MagicMock()
                self.destroy = MagicMock()
            def recv(self):
                return self.q.get(block=True)

        with patch('peerprint.wan.comms.zmq') as z:
            self.notify = queue.Queue()
            self.cb = MagicMock(side_effect=lambda arg: self.notify.put(arg))
            self.pull = DummySock()
            self.req = DummySock()
            z.Context().socket.side_effect = [self.req, self.pull]
            self.c = ZMQClient("reqaddr", "pulladdr", self.cb, logging.getLogger())
            self.req.connect.assert_called_once()
            self.pull.bind.assert_called_once()

    def tearDown(self):
        if self.c is not None:
            self.pull.closed = True
            self.c.destroy()

    def testReceive(self):
        v = spb.Ok() # simple test message
        apb = Any()
        apb.Pack(v)
        self.pull.q.put(apb.SerializeToString())
        self.notify.get()
        self.cb.assert_called_once_with(v)

    def testReceiveUnknownMsg(self):
        v = StringValue(value="test")
        apb = Any()
        apb.Pack(v)
        self.pull.q.put(apb.SerializeToString())
        e = self.notify.get()
        self.cb.assert_called_once()
        self.assertTrue(str(e).startswith("Could not unpack"))
    
    def testCall(self):
        rep = spb.Ok() # simple test reply
        apb = Any()
        apb.Pack(rep)
        self.req.q.put(apb.SerializeToString())

        req = StringValue(value="test")
        apb2 = Any()
        apb2.Pack(req)

        got = self.c.call(req)
        self.req.send.assert_called_with(apb2.SerializeToString())
        self.assertEqual(got, rep)

    def testDestroy(self):
        self.c.destroy()
        self.c = None
        self.pull.destroy.assert_called_once()
        self.req.destroy.assert_called_once()
