# -*- coding: utf-8 -*-
# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: proto/state.proto
"""Generated protocol buffer code."""
from google.protobuf.internal import builder as _builder
from google.protobuf import descriptor as _descriptor
from google.protobuf import descriptor_pool as _descriptor_pool
from google.protobuf import symbol_database as _symbol_database
# @@protoc_insertion_point(imports)

_sym_db = _symbol_database.Default()


from peerprint.server.proto import jobs_pb2 as proto_dot_jobs__pb2


DESCRIPTOR = _descriptor_pool.Default().AddSerializedFile(b'\n\x11proto/state.proto\x12\x05state\x1a\x10proto/jobs.proto\"e\n\x05State\x12$\n\x04jobs\x18\x01 \x03(\x0b\x32\x16.state.State.JobsEntry\x1a\x36\n\tJobsEntry\x12\x0b\n\x03key\x18\x01 \x01(\t\x12\x18\n\x05value\x18\x02 \x01(\x0b\x32\t.jobs.Job:\x02\x38\x01\"\x17\n\x05\x45rror\x12\x0e\n\x06status\x18\x02 \x01(\t\"\x04\n\x02OkB.Z,github.com/smartin015/peerprint/pubsub/protob\x06proto3')

_builder.BuildMessageAndEnumDescriptors(DESCRIPTOR, globals())
_builder.BuildTopDescriptorsAndMessages(DESCRIPTOR, 'proto.state_pb2', globals())
if _descriptor._USE_C_DESCRIPTORS == False:

  DESCRIPTOR._options = None
  DESCRIPTOR._serialized_options = b'Z,github.com/smartin015/peerprint/pubsub/proto'
  _STATE_JOBSENTRY._options = None
  _STATE_JOBSENTRY._serialized_options = b'8\001'
  _STATE._serialized_start=46
  _STATE._serialized_end=147
  _STATE_JOBSENTRY._serialized_start=93
  _STATE_JOBSENTRY._serialized_end=147
  _ERROR._serialized_start=149
  _ERROR._serialized_end=172
  _OK._serialized_start=174
  _OK._serialized_end=178
# @@protoc_insertion_point(module_scope)
