import unittest
import logging
from unittest.mock import MagicMock, ANY
from .lan_queue import LANPrintQueue
from .sync_objects_test import TestReplDict

logging.basicConfig(level=logging.DEBUG)

class MockLockManager():
    def __init__(self, selfID=None):
        self.id = selfID
        self.locks = {}

    def tryAcquire(self, lockid, sync=False, timeout=0):
        if lockid in self.locks:
            return False
        self.locks[self.id] = [lockid]
        return True

    def release(self, lockid):
        for peer, locks in self.locks.items():
            locks.remove(lockid)

    def getPeerLocks(self):
        return self.locks

class LANQueueLocalTest():
    def setUp(self):
        self.addr = "localhost:6789"
        self.manifest = {"man": "ifest"}
        cb = MagicMock()
        self.q = LANPrintQueue("ns", self.addr, cb, logging.getLogger())
        self.q._init_base()
        self.q.q._syncobj = MagicMock()
        self.q.q.peers = TestReplDict(cb)
        self.q.q.jobs = TestReplDict(cb)
        self.q.q.locks = MockLockManager(self.addr)
    
    def tearDown(self):
        self.q.destroy()



class TestLanQueueInitExceptions(unittest.TestCase):
    def test_init_privileged_port(self):
        with self.assertRaises(ValueError):
            LANPrintQueue("ns", "locahost:80", None, logging.getLogger())

    def test_init_bad_addr(self):
        with self.assertRaises(ValueError):
            LANPrintQueue("ns", "hi", None, logging.getLogger())

class TestLanQueuePreStartup(unittest.TestCase):
    def setUp(self):
        self.q = LANPrintQueue("ns", "localhost:6789", MagicMock(), logging.getLogger())

    def tearDown(self):
        self.q.destroy()

    def test_startup_with_no_peers(self):
        self.q._init_base() # Don't call on_startup_complete as it actually does networking
        self.assertNotEqual(self.q.q, None)

    def test_startup_host_added(self):
        self.q._on_host_added("doesnothing") # Verifies no errors due to queue not initialized
        self.q._init_base()
        self.assertNotEqual(self.q.q, None)

class TestLanQueueOperations(LANQueueLocalTest, unittest.TestCase):
    def test_peer_added_after_startup(self):
        self.q._on_host_added("peer1")
        self.q.q._syncobj.addNodeToCluster.assert_called_with("peer1", callback=ANY)
        
    def test_peer_removed_after_startup(self):
        self.q._on_host_removed("peer1")
        self.q.q._syncobj.removeNodeFromCluster.assert_called_with("peer1", callback=ANY)

    def test_job_operations(self):
        self.q.q.setJob("hash", self.manifest)
        self.assertEqual(self.q.q.jobs["hash"], (self.addr, self.manifest))
        self.q.q.acquireJob("hash")
        self.assertEqual(self.q.q.locks.getPeerLocks()[self.addr][0], 'hash')
        self.q.q.releaseJob("hash")
        self.assertEqual(len(self.q.q.locks.getPeerLocks()[self.addr]), 0)
        self.q.q.removeJob("hash")
        self.assertEqual(len(self.q.q.jobs), 0)

    def testPeerGettersHideTimestamp(self):
        self.q.q.syncPeer(dict(a=1), "addr1")
        self.assertEqual(self.q.q.getPeers(), {"addr1": {"a": 1}})
        self.assertEqual(self.q.q.getPeer("addr1"), {"a": 1})

    def testJobGettersIncludePeer(self):
        self.q.q.setJob("job1", dict(a=1), addr="1.2.3.4")
        self.assertEqual(list(self.q.q.getJobs()), [("job1", ("1.2.3.4", {"a": 1}))])
        self.assertEqual(self.q.q.getJob("job1"), ("1.2.3.4", {"a": 1}))
