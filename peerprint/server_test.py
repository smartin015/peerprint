import unittest
from enum import IntEnum
import logging
from unittest.mock import MagicMock
from .proc import ServerProcessOpts
import peerprint.pkg.proto.state_pb2 as spb
import peerprint.pkg.proto.peers_pb2 as ppb
import peerprint.pkg.proto.command_pb2 as cpb

logging.basicConfig(level=logging.DEBUG)

class ObjectCodec:
    @classmethod
    def encode(self, data):
        return b"", "object"

    @classmethod
    def decode(self, data, protocol):
        assert protocol=="object"
        return dict()

class MockServer:
    class CompletionType(IntEnum):
        UNKNOWN = spb.UNKNOWN_COMPLETION_TYPE
        ACQUIRE = spb.ACQUIRE
        RELEASE = spb.RELEASE
        TOMBSTONE = spb.TOMBSTONE

    def __init__(self, peers):
        self.peers = peers
        # Just keyed by uuid, although real schema is by uuid+signer
        self.records = {}
        self.completions = {}
        self.status = None

    def is_ready(self):
        return True

    def get_id(self, network):
        return "MOCKID"

    def set_status(self, network, **kwargs):
        self.status = kwargs

    def get_peers(self, ns):
        return self.peers

    def set_record(self, ns, **kwargs):
        rank = spb.Rank(**kwargs['rank'])
        del kwargs['rank']
        rec = spb.Record(**kwargs, rank=rank)
        self.records[rec.uuid] = rec

    def set_completion(self, ns, **kwargs):
        cpl = spb.Completion(**kwargs)
        self.completions[cpl.uuid] = cpl

    def get_records(self, net, uuid=None):
        return [spb.SignedRecord(record=r) for r in self.records.values() if uuid is None or r.uuid == uuid]
    
    def get_completions(self, net, uuid=None):
        return [spb.SignedCompletion(completion=c) for c in self.completions.values() if uuid is None or c.uuid == uuid]

