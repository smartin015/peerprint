// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.5
// source: proto/peers.proto

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

type PeerType int32

const (
	PeerType_UNKNOWN_PEER_TYPE PeerType = 0
	PeerType_LISTENER          PeerType = 2 // Not involved in consensus
	PeerType_ELECTABLE         PeerType = 3 // A potential LEADER
	PeerType_LEADER            PeerType = 4 // Handles all authorization-requiring activity on the network
)

// Enum value maps for PeerType.
var (
	PeerType_name = map[int32]string{
		0: "UNKNOWN_PEER_TYPE",
		2: "LISTENER",
		3: "ELECTABLE",
		4: "LEADER",
	}
	PeerType_value = map[string]int32{
		"UNKNOWN_PEER_TYPE": 0,
		"LISTENER":          2,
		"ELECTABLE":         3,
		"LEADER":            4,
	}
)

func (x PeerType) Enum() *PeerType {
	p := new(PeerType)
	*p = x
	return p
}

func (x PeerType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (PeerType) Descriptor() protoreflect.EnumDescriptor {
	return file_proto_peers_proto_enumTypes[0].Descriptor()
}

func (PeerType) Type() protoreflect.EnumType {
	return &file_proto_peers_proto_enumTypes[0]
}

func (x PeerType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use PeerType.Descriptor instead.
func (PeerType) EnumDescriptor() ([]byte, []int) {
	return file_proto_peers_proto_rawDescGZIP(), []int{0}
}

// PeerStatus is sent by a peer to indicate its role and addresses
// When type is UNKNOWN, the RAFT leader is supposed to assign a role.
// Note the ID of the peer is passed along and signed cryptographically
// as part of the underlying pubsub implementation
type PeerStatus struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The role of the peer
	Type PeerType `protobuf:"varint,1,opt,name=type,proto3,enum=peers.PeerType" json:"type,omitempty"`
	// Raft hosting address
	RaftAddr *AddrInfo `protobuf:"bytes,2,opt,name=raft_addr,json=raftAddr,proto3" json:"raft_addr,omitempty"`
}

func (x *PeerStatus) Reset() {
	*x = PeerStatus{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_peers_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PeerStatus) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PeerStatus) ProtoMessage() {}

func (x *PeerStatus) ProtoReflect() protoreflect.Message {
	mi := &file_proto_peers_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PeerStatus.ProtoReflect.Descriptor instead.
func (*PeerStatus) Descriptor() ([]byte, []int) {
	return file_proto_peers_proto_rawDescGZIP(), []int{0}
}

func (x *PeerStatus) GetType() PeerType {
	if x != nil {
		return x.Type
	}
	return PeerType_UNKNOWN_PEER_TYPE
}

func (x *PeerStatus) GetRaftAddr() *AddrInfo {
	if x != nil {
		return x.RaftAddr
	}
	return nil
}

// Matches https://pkg.go.dev/github.com/libp2p/go-libp2p/core/peer#AddrInfo
// using string encoding for id and addrs.
type AddrInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id    string   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Addrs []string `protobuf:"bytes,2,rep,name=addrs,proto3" json:"addrs,omitempty"`
}

func (x *AddrInfo) Reset() {
	*x = AddrInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_peers_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AddrInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AddrInfo) ProtoMessage() {}

func (x *AddrInfo) ProtoReflect() protoreflect.Message {
	mi := &file_proto_peers_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AddrInfo.ProtoReflect.Descriptor instead.
func (*AddrInfo) Descriptor() ([]byte, []int) {
	return file_proto_peers_proto_rawDescGZIP(), []int{1}
}

func (x *AddrInfo) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *AddrInfo) GetAddrs() []string {
	if x != nil {
		return x.Addrs
	}
	return nil
}

// A directive sent by the leader of the topic to tell the peer how to collaborate
type AssignPeer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// ID of the target peer
	Peer string `protobuf:"bytes,1,opt,name=peer,proto3" json:"peer,omitempty"`
	// Role of the peer on this topic
	Type PeerType `protobuf:"varint,2,opt,name=type,proto3,enum=peers.PeerType" json:"type,omitempty"`
	// All grants necessary for bootstrapping authority
	// These are copied from the current leader's DB
	Grants []*SignedGrant `protobuf:"bytes,3,rep,name=grants,proto3" json:"grants,omitempty"`
}

func (x *AssignPeer) Reset() {
	*x = AssignPeer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_peers_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AssignPeer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AssignPeer) ProtoMessage() {}

func (x *AssignPeer) ProtoReflect() protoreflect.Message {
	mi := &file_proto_peers_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AssignPeer.ProtoReflect.Descriptor instead.
func (*AssignPeer) Descriptor() ([]byte, []int) {
	return file_proto_peers_proto_rawDescGZIP(), []int{2}
}

func (x *AssignPeer) GetPeer() string {
	if x != nil {
		return x.Peer
	}
	return ""
}

func (x *AssignPeer) GetType() PeerType {
	if x != nil {
		return x.Type
	}
	return PeerType_UNKNOWN_PEER_TYPE
}

func (x *AssignPeer) GetGrants() []*SignedGrant {
	if x != nil {
		return x.Grants
	}
	return nil
}

var File_proto_peers_proto protoreflect.FileDescriptor

var file_proto_peers_proto_rawDesc = []byte{
	0x0a, 0x11, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70, 0x65, 0x65, 0x72, 0x73, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x05, 0x70, 0x65, 0x65, 0x72, 0x73, 0x1a, 0x11, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2f, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x5f, 0x0a,
	0x0a, 0x50, 0x65, 0x65, 0x72, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x23, 0x0a, 0x04, 0x74,
	0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0f, 0x2e, 0x70, 0x65, 0x65, 0x72,
	0x73, 0x2e, 0x50, 0x65, 0x65, 0x72, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65,
	0x12, 0x2c, 0x0a, 0x09, 0x72, 0x61, 0x66, 0x74, 0x5f, 0x61, 0x64, 0x64, 0x72, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x70, 0x65, 0x65, 0x72, 0x73, 0x2e, 0x41, 0x64, 0x64, 0x72,
	0x49, 0x6e, 0x66, 0x6f, 0x52, 0x08, 0x72, 0x61, 0x66, 0x74, 0x41, 0x64, 0x64, 0x72, 0x22, 0x30,
	0x0a, 0x08, 0x41, 0x64, 0x64, 0x72, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x14, 0x0a, 0x05, 0x61, 0x64,
	0x64, 0x72, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x05, 0x61, 0x64, 0x64, 0x72, 0x73,
	0x22, 0x71, 0x0a, 0x0a, 0x41, 0x73, 0x73, 0x69, 0x67, 0x6e, 0x50, 0x65, 0x65, 0x72, 0x12, 0x12,
	0x0a, 0x04, 0x70, 0x65, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x65,
	0x65, 0x72, 0x12, 0x23, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x0f, 0x2e, 0x70, 0x65, 0x65, 0x72, 0x73, 0x2e, 0x50, 0x65, 0x65, 0x72, 0x54, 0x79, 0x70,
	0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x2a, 0x0a, 0x06, 0x67, 0x72, 0x61, 0x6e, 0x74,
	0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e,
	0x53, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x47, 0x72, 0x61, 0x6e, 0x74, 0x52, 0x06, 0x67, 0x72, 0x61,
	0x6e, 0x74, 0x73, 0x2a, 0x4a, 0x0a, 0x08, 0x50, 0x65, 0x65, 0x72, 0x54, 0x79, 0x70, 0x65, 0x12,
	0x15, 0x0a, 0x11, 0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57, 0x4e, 0x5f, 0x50, 0x45, 0x45, 0x52, 0x5f,
	0x54, 0x59, 0x50, 0x45, 0x10, 0x00, 0x12, 0x0c, 0x0a, 0x08, 0x4c, 0x49, 0x53, 0x54, 0x45, 0x4e,
	0x45, 0x52, 0x10, 0x02, 0x12, 0x0d, 0x0a, 0x09, 0x45, 0x4c, 0x45, 0x43, 0x54, 0x41, 0x42, 0x4c,
	0x45, 0x10, 0x03, 0x12, 0x0a, 0x0a, 0x06, 0x4c, 0x45, 0x41, 0x44, 0x45, 0x52, 0x10, 0x04, 0x42,
	0x2e, 0x5a, 0x2c, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x6d,
	0x61, 0x72, 0x74, 0x69, 0x6e, 0x30, 0x31, 0x35, 0x2f, 0x70, 0x65, 0x65, 0x72, 0x70, 0x72, 0x69,
	0x6e, 0x74, 0x2f, 0x70, 0x75, 0x62, 0x73, 0x75, 0x62, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_peers_proto_rawDescOnce sync.Once
	file_proto_peers_proto_rawDescData = file_proto_peers_proto_rawDesc
)

func file_proto_peers_proto_rawDescGZIP() []byte {
	file_proto_peers_proto_rawDescOnce.Do(func() {
		file_proto_peers_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_peers_proto_rawDescData)
	})
	return file_proto_peers_proto_rawDescData
}

var file_proto_peers_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_proto_peers_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_proto_peers_proto_goTypes = []interface{}{
	(PeerType)(0),       // 0: peers.PeerType
	(*PeerStatus)(nil),  // 1: peers.PeerStatus
	(*AddrInfo)(nil),    // 2: peers.AddrInfo
	(*AssignPeer)(nil),  // 3: peers.AssignPeer
	(*SignedGrant)(nil), // 4: state.SignedGrant
}
var file_proto_peers_proto_depIdxs = []int32{
	0, // 0: peers.PeerStatus.type:type_name -> peers.PeerType
	2, // 1: peers.PeerStatus.raft_addr:type_name -> peers.AddrInfo
	0, // 2: peers.AssignPeer.type:type_name -> peers.PeerType
	4, // 3: peers.AssignPeer.grants:type_name -> state.SignedGrant
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_proto_peers_proto_init() }
func file_proto_peers_proto_init() {
	if File_proto_peers_proto != nil {
		return
	}
	file_proto_state_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_proto_peers_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PeerStatus); i {
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
		file_proto_peers_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AddrInfo); i {
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
		file_proto_peers_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AssignPeer); i {
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
			RawDescriptor: file_proto_peers_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proto_peers_proto_goTypes,
		DependencyIndexes: file_proto_peers_proto_depIdxs,
		EnumInfos:         file_proto_peers_proto_enumTypes,
		MessageInfos:      file_proto_peers_proto_msgTypes,
	}.Build()
	File_proto_peers_proto = out.File
	file_proto_peers_proto_rawDesc = nil
	file_proto_peers_proto_goTypes = nil
	file_proto_peers_proto_depIdxs = nil
}
