import unittest
import logging
from unittest.mock import MagicMock, call, patch, ANY
from .registry import IPFSRegistry
import peerprint.server.proto.peers_pb2 as ppb
import yaml
import json
from google.protobuf.json_format import MessageToJson


class TestRegistry(unittest.TestCase):
    def _writeReg(self, reg, dest):
        with open(dest, 'w') as f:
            yaml.dump(json.loads(MessageToJson(reg)), f)
            print("Dumped yaml to", dest)

    def testGetTrustedPeersAndRendezvous(self):
        reg = ppb.Registry(queues=[
            ppb.Queue(name="q1", trustedPeers=["p1_1", "p1_2"], rendezvous="r1"),
            ppb.Queue(name="q2", trustedPeers=["p2_1", "p2_2"], rendezvous="r2"),
            ])
        with patch('peerprint.wan.registry.IPFS') as fs:
            fs.fetch.side_effect = lambda cid, dest: self._writeReg(reg, dest)
            r = IPFSRegistry("testcid")
        
            self.assertEqual(r.get_rendezvous("q2"), reg.queues[1].rendezvous)
            self.assertEqual(r.get_trusted_peers("q1"), reg.queues[0].trustedPeers)

