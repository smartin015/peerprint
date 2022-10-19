# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: proto/peers.proto
"""Generated protocol buffer code."""
from google.protobuf.internal import builder as _builder
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import symbol_database as _symbol_database
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()




DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x11proto/peers.proto\x12\x03rpc\"Z\n\x05Queue\x12\x0c\n\x04name\x18\x01 \x01(\t\x12\x0c\n\x04\x64\x65sc\x18\x02 \x01(\t\x12\x0b\n\x03url\x18\x03 \x01(\t\x12\x12\n\nrendezvous\x18\x04 \x01(\t\x12\x14\n\x0ctrustedPeers\x18\x05 \x03(\t\"D\n\x08Registry\x12\x0f\n\x07\x63reated\x18\x01 \x01(\x04\x12\x0b\n\x03url\x18\x02 \x01(\t\x12\x1a\n\x06queues\x18\x03 \x03(\x0b\x32\n.rpc.Queue\"s\n\nPeerStatus\x12\n\n\x02id\x18\x01 \x01(\t\x12\r\n\x05topic\x18\x02 \x01(\t\x12\x0e\n\x06leader\x18\x03 \x01(\t\x12\x1b\n\x04type\x18\x04 \x01(\x0e\x32\r.rpc.PeerType\x12\x1d\n\x05state\x18\x05 \x01(\x0e\x32\x0e.rpc.PeerState\"X\n\x0cPeersSummary\x12\x15\n\rpeer_estimate\x18\x01 \x01(\x03\x12\x10\n\x08variance\x18\x02 \x01(\x01\x12\x1f\n\x06sample\x18\x03 \x03(\x0b\x32\x0f.rpc.PeerStatus\"6\n\x10PollPeersRequest\x12\r\n\x05topic\x18\x01 \x01(\t\x12\x13\n\x0bprobability\x18\x02 \x01(\x01\"4\n\x11PollPeersResponse\x12\x1f\n\x06status\x18\x01 \x01(\x0b\x32\x0f.rpc.PeerStatus\"\x13\n\x11\x41ssignmentRequest\"7\n\x10RaftAddrsRequest\x12\x0f\n\x07raft_id\x18\x01 \x01(\t\x12\x12\n\nraft_addrs\x18\x02 \x03(\t\"8\n\x11RaftAddrsResponse\x12\x0f\n\x07raft_id\x18\x01 \x01(\t\x12\x12\n\nraft_addrs\x18\x02 \x03(\t\"_\n\x12\x41ssignmentResponse\x12\r\n\x05topic\x18\x01 \x01(\t\x12\n\n\x02id\x18\x02 \x01(\t\x12\x11\n\tleader_id\x18\x03 \x01(\t\x12\x1b\n\x04type\x18\x04 \x01(\x0e\x32\r.rpc.PeerType\"\x1f\n\x11NewLeaderResponse\x12\n\n\x02id\x18\x01 \x01(\t*>\n\x08PeerType\x12\x15\n\x11UNKNOWN_PEER_TYPE\x10\x00\x12\r\n\tELECTABLE\x10\x02\x12\x0c\n\x08LISTENER\x10\x03*C\n\tPeerState\x12\x16\n\x12UNKNOWN_PEER_STATE\x10\x00\x12\x08\n\x04\x42USY\x10\x01\x12\x08\n\x04IDLE\x10\x02\x12\n\n\x06PAUSED\x10\x03\x42.Z,github.com/smartin015/peerprint/pubsub/protob\x06proto3')

_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, globals())
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'proto.peers_pb2', globals())
if _descriptor._USE_C_DESCRIPTORS == False:

  DESCRIPTOR._options = None
  DESCRIPTOR._serialized_options = b'Z,github.com/smartin015/peerprint/pubsub/proto'
  _PEERTYPE._serialized_start=771
  _PEERTYPE._serialized_end=833
  _PEERSTATE._serialized_start=835
  _PEERSTATE._serialized_end=902
  _QUEUE._serialized_start=26
  _QUEUE._serialized_end=116
  _REGISTRY._serialized_start=118
  _REGISTRY._serialized_end=186
  _PEERSTATUS._serialized_start=188
  _PEERSTATUS._serialized_end=303
  _PEERSSUMMARY._serialized_start=305
  _PEERSSUMMARY._serialized_end=393
  _POLLPEERSREQUEST._serialized_start=395
  _POLLPEERSREQUEST._serialized_end=449
  _POLLPEERSRESPONSE._serialized_start=451
  _POLLPEERSRESPONSE._serialized_end=503
  _ASSIGNMENTREQUEST._serialized_start=505
  _ASSIGNMENTREQUEST._serialized_end=524
  _RAFTADDRSREQUEST._serialized_start=526
  _RAFTADDRSREQUEST._serialized_end=581
  _RAFTADDRSRESPONSE._serialized_start=583
  _RAFTADDRSRESPONSE._serialized_end=639
  _ASSIGNMENTRESPONSE._serialized_start=641
  _ASSIGNMENTRESPONSE._serialized_end=736
  _NEWLEADERRESPONSE._serialized_start=738
  _NEWLEADERRESPONSE._serialized_end=769
# @@protoc_insertion_point(module_scope)