# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: peers.proto
"""Generated protocol buffer code."""
from google.protobuf.internal import builder as _builder
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import symbol_database as _symbol_database
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()




DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x0bpeers.proto\x12\x05peers\"g\n\nPeerStatus\x12\x0c\n\x04name\x18\x01 \x01(\t\x12\x15\n\ractive_record\x18\x02 \x01(\t\x12\x13\n\x0b\x61\x63tive_unit\x18\x03 \x01(\t\x12\x0e\n\x06status\x18\x04 \x01(\t\x12\x0f\n\x07profile\x18\x05 \x01(\t\"%\n\x08\x41\x64\x64rInfo\x12\n\n\x02id\x18\x01 \x01(\t\x12\r\n\x05\x61\x64\x64rs\x18\x02 \x03(\t\"\x11\n\x0fGetPeersRequest\"6\n\x10GetPeersResponse\x12\"\n\tAddresses\x18\x01 \x03(\x0b\x32\x0f.peers.AddrInfo\"\x12\n\x10GetStatusRequest\"\xa5\x01\n\rNetworkConfig\x12\x0c\n\x04uuid\x18\x01 \x01(\t\x12\x0c\n\x04name\x18\x02 \x01(\t\x12\x13\n\x0b\x64\x65scription\x18\x03 \x01(\t\x12\x0c\n\x04tags\x18\x04 \x03(\t\x12\r\n\x05links\x18\x05 \x03(\t\x12\x10\n\x08location\x18\x06 \x01(\t\x12\x12\n\nrendezvous\x18\x07 \x01(\t\x12\x0f\n\x07\x63reator\x18\t \x01(\t\x12\x0f\n\x07\x63reated\x18\n \x01(\x03\"\x85\x01\n\x0cNetworkStats\x12\x12\n\npopulation\x18\x01 \x01(\x03\x12\x1d\n\x15\x63ompletions_last7days\x18\x02 \x01(\x03\x12\x0f\n\x07records\x18\x03 \x01(\x03\x12\x14\n\x0cidle_records\x18\x04 \x01(\x03\x12\x1b\n\x13\x61vg_completion_time\x18\x05 \x01(\x03\"f\n\x07Network\x12$\n\x06\x63onfig\x18\x01 \x01(\x0b\x32\x14.peers.NetworkConfig\x12\x11\n\tsignature\x18\x02 \x01(\x0c\x12\"\n\x05stats\x18\x03 \x01(\x0b\x32\x13.peers.NetworkStats\".\n\x0ePubKeyExchange\x12\x0e\n\x06pubkey\x18\x01 \x01(\x0c\x12\x0c\n\x04salt\x18\x02 \x01(\x0c\"!\n\x0c\x45ncryptedPSK\x12\x11\n\tencrypted\x18\x01 \x01(\x0c\x42.Z,github.com/smartin015/peerprint/pubsub/protob\x06proto3')

_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, globals())
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'peers_pb2', globals())
if _descriptor._USE_C_DESCRIPTORS == False:

  DESCRIPTOR._options = None
  DESCRIPTOR._serialized_options = b'Z,github.com/smartin015/peerprint/pubsub/proto'
  _PEERSTATUS._serialized_start=22
  _PEERSTATUS._serialized_end=125
  _ADDRINFO._serialized_start=127
  _ADDRINFO._serialized_end=164
  _GETPEERSREQUEST._serialized_start=166
  _GETPEERSREQUEST._serialized_end=183
  _GETPEERSRESPONSE._serialized_start=185
  _GETPEERSRESPONSE._serialized_end=239
  _GETSTATUSREQUEST._serialized_start=241
  _GETSTATUSREQUEST._serialized_end=259
  _NETWORKCONFIG._serialized_start=262
  _NETWORKCONFIG._serialized_end=427
  _NETWORKSTATS._serialized_start=430
  _NETWORKSTATS._serialized_end=563
  _NETWORK._serialized_start=565
  _NETWORK._serialized_end=667
  _PUBKEYEXCHANGE._serialized_start=669
  _PUBKEYEXCHANGE._serialized_end=715
  _ENCRYPTEDPSK._serialized_start=717
  _ENCRYPTEDPSK._serialized_end=750
# @@protoc_insertion_point(module_scope)