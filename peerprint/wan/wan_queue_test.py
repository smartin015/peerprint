import unittest
import logging
from unittest.mock import MagicMock
from .wan_queue import PeerPrintQueue
from .proc import ServerProcessOpts
import peerprint.server.proto.peers_pb2 as ppb
import peerprint.server.proto.state_pb2 as spb
import peerprint.server.proto.jobs_pb2 as jpb

logging.basicConfig(level=logging.DEBUG)

class ObjectCodec:
    @classmethod
    def encode(self, data):
        return b"", "object"

    @classmethod
    def decode(self, data, protocol):
        assert protocol=="object"
        return dict()

class TestPeerPrintQueue(unittest.TestCase):
    def setUp(self):
        self.cb = MagicMock()
        self.q = PeerPrintQueue(
                opts=ServerProcessOpts(),
                peer_id="foo",
                codec=ObjectCodec,
                binary_path="testbinary",
                update_cb=self.cb,
                logger=logging.getLogger(),
                keydir=None,
            )
        self.q._zmqclient = MagicMock()

    def tearDown(self):
        self.q.destroy()

    def testSyncPeer(self):
        self.q.syncPeer(dict(asdf="ghjk"), addr="abc:123")
        self.q._zmqclient.call.assert_called_with(ppb.PeerStatus())

    def testGetPeersNone(self):
        got = self.q.getPeers()
        self.assertEqual(got.sample, [])

    def testGetPeersWithPeerList(self):
        want = ppb.PeersSummary(peer_estimate=1, variance=float('inf'), sample=[ppb.PeerStatus(id="foo")])
        self.q._update(want)
        self.assertEqual(self.q.getPeers(), want)

    def testJobPresenceAndGetters(self):
        s=spb.State(jobs=dict(foo=jpb.Job(id="foo", data=b"", protocol="object", owner='testpeer')))
        self.q._update(s)
        self.assertEqual(self.q.hasJob("foo"), True)
        self.assertEqual(self.q.hasJob("bar"), False)
        self.assertEqual(self.q.getJobs(), {'foo': {'peer_': 'testpeer'}})

    def testSetJob(self):
        self.q.setJob("foo", dict(man="ifest"), addr="testaddr")
        req = self.q._zmqclient.call.call_args[0][0]
        self.assertEqual(req.job.id, "foo")

    def testRemoveJob(self):
        self.q.removeJob("foo")
        self.q._zmqclient.call.assert_called_with(jpb.DeleteJobRequest(id="foo"))

    def testAcquireJob(self):
        self.q.acquireJob("foo")
        self.q._zmqclient.call.assert_called_with(jpb.AcquireJobRequest(id="foo"))

    def testReleaseJob(self):
        self.q.releaseJob("foo")
        self.q._zmqclient.call.assert_called_with(jpb.ReleaseJobRequest(id="foo"))
