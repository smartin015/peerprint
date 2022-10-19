import unittest
import logging
from unittest.mock import MagicMock
from .wan_queue import PeerPrintQueue
import peerprint.server.proto.peers_pb2 as ppb

logging.basicConfig(level=logging.DEBUG)

class TestPeerPrintQueue(unittest.TestCase):
    def setUp(self):
        self.q = PeerPrintQueue("test_ns", logging.getLogger(), lan=False)
        self.q._zmqclient = MagicMock()

    def testSyncPeer(self):
        self.q.syncPeer(dict(asdf="ghjk"), addr="abc:123")
        self.q._zmqclient.call.assert_called_with(ppb.PeerStatus()) #state=dict(asdf="ghjk"), addr="abc:123"))

    def testGetPeersNone(self):
        pass # TODO

    def testGetPeersWithPeerList(self):
        pass # TODO

    def testHasJobTrue(self):
        pass # TODO

    def testHasJobFalse(self):
        pass # TODO

    def testSetJob(self):
        pass # TODO

    def testGetLocks(self):
        pass # TODO

    def testGetJobs(self):
        pass # TODO

    def testGetJob(self):
        pass # TODO

    def testRemoveJob(self):
        pass # TODO

    def testAcquireJob(self):
        pass # TODO

    def testReleaseJob(self):
        pass # TODO
