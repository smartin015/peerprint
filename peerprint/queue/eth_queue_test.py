from .eth_queue import EthereumQueue, compatify
import unittest
from unittest.mock import MagicMock

ACC = 'ACCOUNTHASH'

MF = dict(profiles=['a','b'], materials=['c','d'])
CF = compatify(MF['profiles'], MF['materials'])

class EthereumQueueTest(unittest.TestCase):

    def setUp(self):
        self.eq = EthereumQueue(ACC)
        self.eq._contract = MagicMock()
        self.eq._contract.functions.getOwnJobs().call.return_value = [1,1,0]
        self.eq._contract.functions.jobs().call.return_value = [1,0]
        self.eq._w3 = MagicMock()

    def testSetJob(self):
        self.eq.setJob("HASH", MF, value=5)
        self.eq._contract.functions.postJob.assert_called_with("HASH", 2, 1, CF)



