import unittest
import logging
from unittest.mock import MagicMock, ANY
from .lan_queue import LANPrintQueue

logging.basicConfig(level=logging.DEBUG)

class TestLANQueueInit(unittest.TestCase):
    def test_init_privileged_port(self):
        with self.assertRaises(ValueError):
            LANPrintQueue("ns", "locahost:80", None, None, logging.getLogger(), testing=True)

    def test_init_bad_addr(self):
        with self.assertRaises(ValueError):
            LANPrintQueue("ns", "hi", None, None, logging.getLogger(), testing=True)

class TestLANQueuePeers(unittest.TestCase):
    def setUp(self):
        self.q = LANPrintQueue("ns", "localhost:6789", "basedir", MagicMock(), logging.getLogger(), testing=True)

    def tearDown(self):
        self.q.destroy()

    def test_startup_with_no_peers(self):
        self.q._on_startup_complete({})
        self.assertNotEqual(self.q.q, None)

    def test_startup_with_discovered_peers(self):
        self.q._on_host_added("doesnothing") # Verifies no errors due to queue not initialized
        self.q._on_startup_complete({"peer1": True, "peer2": True})
        self.assertNotEqual(self.q.q, None)

    def test_peer_added_after_startup(self):
        self.q._on_startup_complete({})
        self.q.q.peers = {}
        self.q.q.addNodeToCluster = MagicMock()
        self.q._on_host_added("peer1")
        self.q.q.addNodeToCluster.assert_called_with("peer1", callback=ANY)
        
    def test_peer_removed_after_startup(self):
        self.q._on_startup_complete({"peer1": True})
        self.q.q.peers = {}
        self.q.q.removeNodeFromCluster = MagicMock()
        self.q._on_host_removed("peer1")
        self.q.q.removeNodeFromCluster.assert_called_with("peer1", callback=ANY)
    
class TestLanQueueOperations(unittest.TestCase):
    def setUp(self):
        self.addr = "localhost:6789"
        self.manifest = {"man": "ifest"}
        self.q = LANPrintQueue("ns", self.addr, "basedir", MagicMock(), logging.getLogger(), testing=True)
        self.q._on_startup_complete({})
        # Replace pysyncobj objects with non-network equivalents
        self.q.q.jobs = {}
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
