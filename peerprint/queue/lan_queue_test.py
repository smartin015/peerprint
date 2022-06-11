import unittest
import logging
from unittest.mock import MagicMock, ANY
from .lan_queue import LANPrintQueue

logging.basicConfig(level=logging.DEBUG)

class MockJobList(dict):
    def set(self, k, v, **kwargs):
        self[k] = v

class TestLANQueueInit(unittest.TestCase):
    def test_init_privileged_port(self):
        with self.assertRaises(ValueError):
            LANPrintQueue("ns", "locahost:80", None, logging.getLogger())

    def test_init_bad_addr(self):
        with self.assertRaises(ValueError):
            LANPrintQueue("ns", "hi", None, logging.getLogger())

class TestLANQueuePeers(unittest.TestCase):
    def setUp(self):
        self.q = LANPrintQueue("ns", "localhost:6789", MagicMock(), logging.getLogger())

    def tearDown(self):
        self.q.destroy()

    def test_startup_with_no_peers(self):
        self.q._init_base({}) # Don't call on_startup_complete as it actually does networking
        self.assertNotEqual(self.q.q, None)

    def test_startup_with_discovered_peers(self):
        self.q._on_host_added("doesnothing") # Verifies no errors due to queue not initialized
        self.q._init_base({"peer1": True, "peer2": True})
        self.assertNotEqual(self.q.q, None)

    def test_peer_added_after_startup(self):
        self.q._init_base({})
        self.q.q.peers = {}
        self.q.q._syncobj = MagicMock()
        self.q._on_host_added("peer1")
        self.q.q._syncobj.addNodeToCluster.assert_called_with("peer1", callback=ANY)
        
    def test_peer_removed_after_startup(self):
        self.q._init_base({"peer1": True})
        self.q.q.peers = {}
        self.q.q._syncobj = MagicMock()
        self.q._on_host_removed("peer1")
        self.q.q._syncobj.removeNodeFromCluster.assert_called_with("peer1", callback=ANY)

class TestLanQueueOperations(unittest.TestCase):
    def setUp(self):
        self.addr = "localhost:6789"
        self.manifest = {"man": "ifest"}
        self.q = LANPrintQueue("ns", self.addr, MagicMock(), logging.getLogger())
        self.q._init_base({})
        # Replace pysyncobj objects with non-network equivalents
        self.q.q.jobs = MockJobList()
        self.q.q.peers = {}
        self.q.q.locks = MagicMock()

    def tearDown(self):
        self.q.destroy()

    def test_job_operations(self):
        self.q.q.setJob("hash", self.manifest)
        self.assertEqual(self.q.q.jobs["hash"], (self.addr, self.manifest))
        self.q.q.acquireJob("hash")
        self.q.q.locks.tryAcquire.assert_called_with("hash", sync=True, timeout=ANY)
        self.q.q.releaseJob("hash")
        self.q.q.locks.release.assert_called_with("hash")
        self.q.q.removeJob("hash")
        self.assertEqual(self.q.q.jobs, {})
