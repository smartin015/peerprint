import unittest
from unittest.mock import MagicMock, patch
from .ipfs import IPFS

class TestIPFS(unittest.TestCase):
    def setUp(self):
        with patch('peerprint.wan.ipfs.IPFSDaemonProcess') as dp:
            dp.is_running.return_value = True
            IPFS.start_daemon()

    def tearDown(self):
        IPFS.stop_daemon()

    def testAdd(self):
        with patch('peerprint.wan.ipfs.subprocess') as subp:
            subp.run.return_value = MagicMock(returncode=0, stdout="test_cid")
            self.assertEqual(IPFS.add("testpath"), "test_cid")
            subp.run.assert_called_once()
            self.assertEqual(subp.run.call_args[0][0][:2], ["ipfs", "add"])

    def testPin(self):
        with patch('peerprint.wan.ipfs.subprocess') as subp:
            subp.run.return_value = MagicMock(returncode=0)
            self.assertTrue(IPFS.pin("testpath"))
            subp.run.assert_called_once()
            self.assertEqual(subp.run.call_args[0][0][:3], ["ipfs", "pin", "add"])

    def testUnpin(self):
        with patch('peerprint.wan.ipfs.subprocess') as subp:
            subp.run.return_value = MagicMock(returncode=0)
            self.assertTrue(IPFS.unpin("testpath"))
            subp.run.assert_called_once()
            self.assertEqual(subp.run.call_args[0][0][:3], ["ipfs", "pin", "rm"])

    def testStat(self):
        with patch('peerprint.wan.ipfs.subprocess') as subp:
            subp.run.return_value = MagicMock(returncode=0, stdout="A:5\nB:10\nC:7".encode("utf8"))
            self.assertEqual(IPFS.stat("testpath"), dict(A=5, B=10, C=7))
            subp.run.assert_called_once()
            self.assertEqual(subp.run.call_args[0][0][:3], ["ipfs", "object", "stat"])

    def testFetch(self):
        with patch('peerprint.wan.ipfs.subprocess') as subp:
            subp.run.side_effect = [
                    MagicMock(returncode=0, stdout=f"CumulativeSize:{IPFS.MAX_CUMULATIVE_SIZE}".encode('utf8')),
                    MagicMock(returncode=0),
            ]
            self.assertTrue(IPFS.fetch("testcid", "testdest"))
            self.assertEqual(subp.run.call_args[0][0][:2], ["ipfs", "get"])

    def testFetchFailureTooLarge(self):
        with patch('peerprint.wan.ipfs.subprocess') as subp:
            subp.run.return_value = MagicMock(returncode=0, stdout=f"CumulativeSize:{IPFS.MAX_CUMULATIVE_SIZE+1}".encode('utf8'))
            with self.assertRaises(Exception):
                IPFS.fetch("testcid", "testdest")

    def testFetchStr(self):
        with patch('peerprint.wan.ipfs.subprocess') as subp:
            subp.run.return_value = MagicMock(returncode=0, stdout="test".encode('utf8'))
            self.assertEqual(IPFS.fetchStr("testcid"), "test")
            subp.run.assert_called_once()
            self.assertEqual(subp.run.call_args[0][0][:2], ["ipfs", "cat"])
