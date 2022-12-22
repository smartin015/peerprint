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


class MockPPQ:
    def __init__(self, opts, codec, binpath, on_update, logger, keydir):
        self.opts = opts
        self.jobs = {}

    def syncPeer(self, profile, state):
        pass # TODO

    def getPeers(self):
        return MagicMock(peer_estimate=0, variance=float('inf'), sample=[
                MagicMock(id="peer1", topic="/", leader="leader", profile="profile", type=1, state=2),
            ])

    def getJobs(self):
        return self.jobs

    def setJob(self, jid, j, addr):
        # Matching behavior of wan_queue.py impl; peer_, acquired, acquired_by_
        # all set when received from process
        j['peer_'] = addr or "dummyppq"
        j['acquired'] = j.get('acquired', False)
        j['acquired_by_'] = j.get('acquired_by_', None)
        self.jobs[jid] = j

    def removeJob(self, jid):
        if not jid in self.jobs:
            return dict(jobs_deleted=0)
        del self.jobs[jid]
        return dict(jobs_deleted=1)

    def acquireJob(self, jid):
        self.jobs[jid]['acquired'] = True
        return True

    def releaseJob(self, jid):
        self.jobs[jid]['acquired'] = False
        return True


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
