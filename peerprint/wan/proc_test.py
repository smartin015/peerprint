import unittest
import logging
import signal
from unittest.mock import MagicMock, call, patch, ANY
from .proc import ServerProcessOpts, DependentProcess, IPFSDaemonProcess, ServerProcess

logging.basicConfig()

class TestOpts(unittest.TestCase):
    def testRender(self):
        opts = ServerProcessOpts(addr="foo", local=True)
        self.assertEqual(opts.render("bar"), ["bar", "-addr=foo", "-local=True"])

class TestDependentProcess(unittest.TestCase):
    def testDestroy(self):
        dp = DependentProcess()
        dp._proc = MagicMock(returncode=None)
        dp._logger = logging.getLogger()
        dp.destroy()
        dp._proc.send_signal.assert_has_calls([
            call(signal.SIGINT),
            call(signal.SIGKILL),
            call(signal.SIGTERM),
            ])

    def testDestroyNoProcess(self):
        dp = DependentProcess()
        dp.destroy()

class TestIPFSDaemonProcess(unittest.TestCase):
    def testInit(self):
        with patch('peerprint.wan.proc.subprocess') as subp:
            p = MagicMock(returncode=1) # Return error when calling `ipfs stats bw` to check server state, indicates not running
            subp.Popen.return_value = p
            d = IPFSDaemonProcess(logging.getLogger())
            self.assertEqual(subp.Popen.call_args[0][0][:2], ["ipfs", "daemon"])
    
    def testInitAlreadyRunning(self):
        with patch('peerprint.wan.proc.subprocess') as subp:
            p = MagicMock(returncode=0) # Return success when calling `ipfs stats bw`, indicates server is running
            subp.Popen.return_value = p
            d = IPFSDaemonProcess(logging.getLogger())
            subp.Popen.assert_called_once_with(["ipfs", "stats", "bw"], stdout=ANY, stderr=ANY)

class TestServerProcses(unittest.TestCase):
    def testInit(self):
        with patch('peerprint.wan.proc.subprocess') as subp:
            subp.Popen.return_value = None
            s = ServerProcess(ServerProcessOpts(addr="foo", local=True), "bin", logging.getLogger())
            subp.Popen.assert_called_once() # options rendering tested earlier in this file
