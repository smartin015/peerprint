// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.5
// source: proto/state.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type LocationType int32

const (
	LocationType_UNKNOWN  LocationType = 0
	LocationType_IPFS_CID LocationType = 1
)

// Enum value maps for LocationType.
var (
	LocationType_name = map[int32]string{
		0: "UNKNOWN",
		1: "IPFS_CID",
	}
	LocationType_value = map[string]int32{
		"UNKNOWN":  0,
		"IPFS_CID": 1,
	}
)

func (x LocationType) Enum() *LocationType {
	p := new(LocationType)
	*p = x
	return p
}

func (x LocationType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (LocationType) Descriptor() protoreflect.EnumDescriptor {
	return file_proto_state_proto_enumTypes[0].Descriptor()
}

func (LocationType) Type() protoreflect.EnumType {
	return &file_proto_state_proto_enumTypes[0]
}

func (x LocationType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use LocationType.Descriptor instead.
func (LocationType) EnumDescriptor() ([]byte, []int) {
	return file_proto_state_proto_rawDescGZIP(), []int{0}
}

type Signature struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Signer string `protobuf:"bytes,1,opt,name=signer,proto3" json:"signer,omitempty"`
	Data   []byte `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *Signature) Reset() {
	*x = Signature{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_state_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Signature) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Signature) ProtoMessage() {}

func (x *Signature) ProtoReflect() protoreflect.Message {
	mi := &file_proto_state_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Signature.ProtoReflect.Descriptor instead.
func (*Signature) Descriptor() ([]byte, []int) {
	return file_proto_state_proto_rawDescGZIP(), []int{0}
}

func (x *Signature) GetSigner() string {
	if x != nil {
		return x.Signer
	}
	return ""
}

func (x *Signature) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type SignedRecord struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Record    *Record    `protobuf:"bytes,1,opt,name=record,proto3" json:"record,omitempty"`
	Signature *Signature `protobuf:"bytes,2,opt,name=signature,proto3" json:"signature,omitempty"`
}

func (x *SignedRecord) Reset() {
	*x = SignedRecord{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_state_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SignedRecord) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignedRecord) ProtoMessage() {}

func (x *SignedRecord) ProtoReflect() protoreflect.Message {
	mi := &file_proto_state_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignedRecord.ProtoReflect.Descriptor instead.
func (*SignedRecord) Descriptor() ([]byte, []int) {
	return file_proto_state_proto_rawDescGZIP(), []int{1}
}

func (x *SignedRecord) GetRecord() *Record {
	if x != nil {
		return x.Record
	}
	return nil
}

func (x *SignedRecord) GetSignature() *Signature {
	if x != nil {
		return x.Signature
	}
	return nil
}

type Rank struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Num uint64  `protobuf:"varint,1,opt,name=num,proto3" json:"num,omitempty"`
	Den uint64  `protobuf:"varint,2,opt,name=den,proto3" json:"den,omitempty"`
	Gen float64 `protobuf:"fixed64,3,opt,name=gen,proto3" json:"gen,omitempty"` // rendered as `num/den`, lossy
}

func (x *Rank) Reset() {
	*x = Rank{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_state_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Rank) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Rank) ProtoMessage() {}

func (x *Rank) ProtoReflect() protoreflect.Message {
	mi := &file_proto_state_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Rank.ProtoReflect.Descriptor instead.
func (*Rank) Descriptor() ([]byte, []int) {
	return file_proto_state_proto_rawDescGZIP(), []int{2}
}

func (x *Rank) GetNum() uint64 {
	if x != nil {
		return x.Num
	}
	return 0
}

func (x *Rank) GetDen() uint64 {
	if x != nil {
		return x.Den
	}
	return 0
}

func (x *Rank) GetGen() float64 {
	if x != nil {
		return x.Gen
	}
	return 0
}

type Record struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Uuid     string   `protobuf:"bytes,1,opt,name=uuid,proto3" json:"uuid,omitempty"`         // UUID unique to this record
	Approver string   `protobuf:"bytes,2,opt,name=approver,proto3" json:"approver,omitempty"` // Who to go to for a Completion
	Tags     []string `protobuf:"bytes,3,rep,name=tags,proto3" json:"tags,omitempty"`         // Tags for qualified search of records without having to fetch each `location`
	Rank     *Rank    `protobuf:"bytes,4,opt,name=rank,proto3" json:"rank,omitempty"`
	Location string   `protobuf:"bytes,5,opt,name=location,proto3" json:"location,omitempty"`
	// Unix timestamps
	Created   int64 `protobuf:"varint,6,opt,name=created,proto3" json:"created,omitempty"`
	Tombstone int64 `protobuf:"varint,7,opt,name=tombstone,proto3" json:"tombstone,omitempty"`
	// Active state information
	Worker      string `protobuf:"bytes,8,opt,name=worker,proto3" json:"worker,omitempty"`
	WorkerState []byte `protobuf:"bytes,9,opt,name=worker_state,json=workerState,proto3" json:"worker_state,omitempty"`
}

func (x *Record) Reset() {
	*x = Record{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_state_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Record) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Record) ProtoMessage() {}

func (x *Record) ProtoReflect() protoreflect.Message {
	mi := &file_proto_state_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Record.ProtoReflect.Descriptor instead.
func (*Record) Descriptor() ([]byte, []int) {
	return file_proto_state_proto_rawDescGZIP(), []int{3}
}

func (x *Record) GetUuid() string {
	if x != nil {
		return x.Uuid
	}
	return ""
}

func (x *Record) GetApprover() string {
	if x != nil {
		return x.Approver
	}
	return ""
}

func (x *Record) GetTags() []string {
	if x != nil {
		return x.Tags
	}
	return nil
}

func (x *Record) GetRank() *Rank {
	if x != nil {
		return x.Rank
	}
	return nil
}

func (x *Record) GetLocation() string {
	if x != nil {
		return x.Location
	}
	return ""
}

func (x *Record) GetCreated() int64 {
	if x != nil {
		return x.Created
	}
	return 0
}

func (x *Record) GetTombstone() int64 {
	if x != nil {
		return x.Tombstone
	}
	return 0
}

func (x *Record) GetWorker() string {
	if x != nil {
		return x.Worker
	}
	return ""
}

func (x *Record) GetWorkerState() []byte {
	if x != nil {
		return x.WorkerState
	}
	return nil
}

type SignedCompletion struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Completion *Completion `protobuf:"bytes,1,opt,name=completion,proto3" json:"completion,omitempty"`
	Signature  *Signature  `protobuf:"bytes,2,opt,name=signature,proto3" json:"signature,omitempty"`
}

func (x *SignedCompletion) Reset() {
	*x = SignedCompletion{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_state_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SignedCompletion) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignedCompletion) ProtoMessage() {}

func (x *SignedCompletion) ProtoReflect() protoreflect.Message {
	mi := &file_proto_state_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignedCompletion.ProtoReflect.Descriptor instead.
func (*SignedCompletion) Descriptor() ([]byte, []int) {
	return file_proto_state_proto_rawDescGZIP(), []int{4}
}

func (x *SignedCompletion) GetCompletion() *Completion {
	if x != nil {
		return x.Completion
	}
	return nil
}

func (x *SignedCompletion) GetSignature() *Signature {
	if x != nil {
		return x.Signature
	}
	return nil
}

type Completion struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Uuid      string `protobuf:"bytes,1,opt,name=uuid,proto3" json:"uuid,omitempty"`
	Completer string `protobuf:"bytes,3,opt,name=completer,proto3" json:"completer,omitempty"`
	Timestamp int64  `protobuf:"varint,4,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
}

func (x *Completion) Reset() {
	*x = Completion{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_state_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Completion) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Completion) ProtoMessage() {}

func (x *Completion) ProtoReflect() protoreflect.Message {
	mi := &file_proto_state_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Completion.ProtoReflect.Descriptor instead.
func (*Completion) Descriptor() ([]byte, []int) {
	return file_proto_state_proto_rawDescGZIP(), []int{5}
}

func (x *Completion) GetUuid() string {
	if x != nil {
		return x.Uuid
	}
	return ""
}

func (x *Completion) GetCompleter() string {
	if x != nil {
		return x.Completer
	}
	return ""
}

func (x *Completion) GetTimestamp() int64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}

var File_proto_state_proto protoreflect.FileDescriptor

var file_proto_state_proto_rawDesc = []byte{
	0x0a, 0x11, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x22, 0x37, 0x0a, 0x09, 0x53, 0x69,
	0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x69, 0x67, 0x6e, 0x65,
	0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x72, 0x12,
	0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64,
	0x61, 0x74, 0x61, 0x22, 0x65, 0x0a, 0x0c, 0x53, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x52, 0x65, 0x63,
	0x6f, 0x72, 0x64, 0x12, 0x25, 0x0a, 0x06, 0x72, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x52, 0x65, 0x63, 0x6f,
	0x72, 0x64, 0x52, 0x06, 0x72, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x12, 0x2e, 0x0a, 0x09, 0x73, 0x69,
	0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x10, 0x2e,
	0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x52,
	0x09, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x22, 0x3c, 0x0a, 0x04, 0x52, 0x61,
	0x6e, 0x6b, 0x12, 0x10, 0x0a, 0x03, 0x6e, 0x75, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52,
	0x03, 0x6e, 0x75, 0x6d, 0x12, 0x10, 0x0a, 0x03, 0x64, 0x65, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x03, 0x64, 0x65, 0x6e, 0x12, 0x10, 0x0a, 0x03, 0x67, 0x65, 0x6e, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x01, 0x52, 0x03, 0x67, 0x65, 0x6e, 0x22, 0xfc, 0x01, 0x0a, 0x06, 0x52, 0x65, 0x63,
	0x6f, 0x72, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x75, 0x75, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x75, 0x75, 0x69, 0x64, 0x12, 0x1a, 0x0a, 0x08, 0x61, 0x70, 0x70, 0x72, 0x6f,
	0x76, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x61, 0x70, 0x70, 0x72, 0x6f,
	0x76, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x61, 0x67, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28,
	0x09, 0x52, 0x04, 0x74, 0x61, 0x67, 0x73, 0x12, 0x1f, 0x0a, 0x04, 0x72, 0x61, 0x6e, 0x6b, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x52, 0x61,
	0x6e, 0x6b, 0x52, 0x04, 0x72, 0x61, 0x6e, 0x6b, 0x12, 0x1a, 0x0a, 0x08, 0x6c, 0x6f, 0x63, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x6c, 0x6f, 0x63, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x18,
	0x06, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x12, 0x1c,
	0x0a, 0x09, 0x74, 0x6f, 0x6d, 0x62, 0x73, 0x74, 0x6f, 0x6e, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28,
	0x03, 0x52, 0x09, 0x74, 0x6f, 0x6d, 0x62, 0x73, 0x74, 0x6f, 0x6e, 0x65, 0x12, 0x16, 0x0a, 0x06,
	0x77, 0x6f, 0x72, 0x6b, 0x65, 0x72, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x77, 0x6f,
	0x72, 0x6b, 0x65, 0x72, 0x12, 0x21, 0x0a, 0x0c, 0x77, 0x6f, 0x72, 0x6b, 0x65, 0x72, 0x5f, 0x73,
	0x74, 0x61, 0x74, 0x65, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0b, 0x77, 0x6f, 0x72, 0x6b,
	0x65, 0x72, 0x53, 0x74, 0x61, 0x74, 0x65, 0x22, 0x75, 0x0a, 0x10, 0x53, 0x69, 0x67, 0x6e, 0x65,
	0x64, 0x43, 0x6f, 0x6d, 0x70, 0x6c, 0x65, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x31, 0x0a, 0x0a, 0x63,
	0x6f, 0x6d, 0x70, 0x6c, 0x65, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x11, 0x2e, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x43, 0x6f, 0x6d, 0x70, 0x6c, 0x65, 0x74, 0x69,
	0x6f, 0x6e, 0x52, 0x0a, 0x63, 0x6f, 0x6d, 0x70, 0x6c, 0x65, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x2e,
	0x0a, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x10, 0x2e, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x74,
	0x75, 0x72, 0x65, 0x52, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x22, 0x5c,
	0x0a, 0x0a, 0x43, 0x6f, 0x6d, 0x70, 0x6c, 0x65, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04,
	0x75, 0x75, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x75, 0x75, 0x69, 0x64,
	0x12, 0x1c, 0x0a, 0x09, 0x63, 0x6f, 0x6d, 0x70, 0x6c, 0x65, 0x74, 0x65, 0x72, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x09, 0x63, 0x6f, 0x6d, 0x70, 0x6c, 0x65, 0x74, 0x65, 0x72, 0x12, 0x1c,
	0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x03, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2a, 0x29, 0x0a, 0x0c,
	0x4c, 0x6f, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x12, 0x0b, 0x0a, 0x07,
	0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57, 0x4e, 0x10, 0x00, 0x12, 0x0c, 0x0a, 0x08, 0x49, 0x50, 0x46,
	0x53, 0x5f, 0x43, 0x49, 0x44, 0x10, 0x01, 0x42, 0x2e, 0x5a, 0x2c, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x6d, 0x61, 0x72, 0x74, 0x69, 0x6e, 0x30, 0x31, 0x35,
	0x2f, 0x70, 0x65, 0x65, 0x72, 0x70, 0x72, 0x69, 0x6e, 0x74, 0x2f, 0x70, 0x75, 0x62, 0x73, 0x75,
	0x62, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_state_proto_rawDescOnce sync.Once
	file_proto_state_proto_rawDescData = file_proto_state_proto_rawDesc
)

func file_proto_state_proto_rawDescGZIP() []byte {
	file_proto_state_proto_rawDescOnce.Do(func() {
		file_proto_state_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_state_proto_rawDescData)
	})
	return file_proto_state_proto_rawDescData
}

var file_proto_state_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_proto_state_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_proto_state_proto_goTypes = []interface{}{
	(LocationType)(0),        // 0: state.LocationType
	(*Signature)(nil),        // 1: state.Signature
	(*SignedRecord)(nil),     // 2: state.SignedRecord
	(*Rank)(nil),             // 3: state.Rank
	(*Record)(nil),           // 4: state.Record
	(*SignedCompletion)(nil), // 5: state.SignedCompletion
	(*Completion)(nil),       // 6: state.Completion
}
var file_proto_state_proto_depIdxs = []int32{
	4, // 0: state.SignedRecord.record:type_name -> state.Record
	1, // 1: state.SignedRecord.signature:type_name -> state.Signature
	3, // 2: state.Record.rank:type_name -> state.Rank
	6, // 3: state.SignedCompletion.completion:type_name -> state.Completion
	1, // 4: state.SignedCompletion.signature:type_name -> state.Signature
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_proto_state_proto_init() }
func file_proto_state_proto_init() {
	if File_proto_state_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_state_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Signature); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_state_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SignedRecord); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_state_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Rank); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_state_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Record); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_state_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SignedCompletion); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_state_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Completion); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto_state_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proto_state_proto_goTypes,
		DependencyIndexes: file_proto_state_proto_depIdxs,
		EnumInfos:         file_proto_state_proto_enumTypes,
		MessageInfos:      file_proto_state_proto_msgTypes,
	}.Build()
	File_proto_state_proto = out.File
	file_proto_state_proto_rawDesc = nil
	file_proto_state_proto_goTypes = nil
	file_proto_state_proto_depIdxs = nil
}