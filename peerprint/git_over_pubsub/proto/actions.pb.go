// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.5
// source: proto/actions.proto

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

type GetStateRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetStateRequest) Reset() {
	*x = GetStateRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_actions_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetStateRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetStateRequest) ProtoMessage() {}

func (x *GetStateRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_actions_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetStateRequest.ProtoReflect.Descriptor instead.
func (*GetStateRequest) Descriptor() ([]byte, []int) {
	return file_proto_actions_proto_rawDescGZIP(), []int{0}
}

type GetStateResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Grants  []*SignedGrant  `protobuf:"bytes,1,rep,name=grants,proto3" json:"grants,omitempty"`
	Records []*SignedRecord `protobuf:"bytes,2,rep,name=records,proto3" json:"records,omitempty"`
}

func (x *GetStateResponse) Reset() {
	*x = GetStateResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_actions_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetStateResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetStateResponse) ProtoMessage() {}

func (x *GetStateResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_actions_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetStateResponse.ProtoReflect.Descriptor instead.
func (*GetStateResponse) Descriptor() ([]byte, []int) {
	return file_proto_actions_proto_rawDescGZIP(), []int{1}
}

func (x *GetStateResponse) GetGrants() []*SignedGrant {
	if x != nil {
		return x.Grants
	}
	return nil
}

func (x *GetStateResponse) GetRecords() []*SignedRecord {
	if x != nil {
		return x.Records
	}
	return nil
}

var File_proto_actions_proto protoreflect.FileDescriptor

var file_proto_actions_proto_rawDesc = []byte{
	0x0a, 0x13, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x07, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x1a, 0x11,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0x11, 0x0a, 0x0f, 0x47, 0x65, 0x74, 0x53, 0x74, 0x61, 0x74, 0x65, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x22, 0x6d, 0x0a, 0x10, 0x47, 0x65, 0x74, 0x53, 0x74, 0x61, 0x74, 0x65,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x2a, 0x0a, 0x06, 0x67, 0x72, 0x61, 0x6e,
	0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x73, 0x74, 0x61, 0x74, 0x65,
	0x2e, 0x53, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x47, 0x72, 0x61, 0x6e, 0x74, 0x52, 0x06, 0x67, 0x72,
	0x61, 0x6e, 0x74, 0x73, 0x12, 0x2d, 0x0a, 0x07, 0x72, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x73, 0x18,
	0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x53, 0x69,
	0x67, 0x6e, 0x65, 0x64, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x52, 0x07, 0x72, 0x65, 0x63, 0x6f,
	0x72, 0x64, 0x73, 0x42, 0x2e, 0x5a, 0x2c, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x73, 0x6d, 0x61, 0x72, 0x74, 0x69, 0x6e, 0x30, 0x31, 0x35, 0x2f, 0x70, 0x65, 0x65,
	0x72, 0x70, 0x72, 0x69, 0x6e, 0x74, 0x2f, 0x70, 0x75, 0x62, 0x73, 0x75, 0x62, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_actions_proto_rawDescOnce sync.Once
	file_proto_actions_proto_rawDescData = file_proto_actions_proto_rawDesc
)

func file_proto_actions_proto_rawDescGZIP() []byte {
	file_proto_actions_proto_rawDescOnce.Do(func() {
		file_proto_actions_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_actions_proto_rawDescData)
	})
	return file_proto_actions_proto_rawDescData
}

var file_proto_actions_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_proto_actions_proto_goTypes = []interface{}{
	(*GetStateRequest)(nil),  // 0: actions.GetStateRequest
	(*GetStateResponse)(nil), // 1: actions.GetStateResponse
	(*SignedGrant)(nil),      // 2: state.SignedGrant
	(*SignedRecord)(nil),     // 3: state.SignedRecord
}
var file_proto_actions_proto_depIdxs = []int32{
	2, // 0: actions.GetStateResponse.grants:type_name -> state.SignedGrant
	3, // 1: actions.GetStateResponse.records:type_name -> state.SignedRecord
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_proto_actions_proto_init() }
func file_proto_actions_proto_init() {
	if File_proto_actions_proto != nil {
		return
	}
	file_proto_state_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_proto_actions_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetStateRequest); i {
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
		file_proto_actions_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetStateResponse); i {
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
			RawDescriptor: file_proto_actions_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proto_actions_proto_goTypes,
		DependencyIndexes: file_proto_actions_proto_depIdxs,
		MessageInfos:      file_proto_actions_proto_msgTypes,
	}.Build()
	File_proto_actions_proto = out.File
	file_proto_actions_proto_rawDesc = nil
	file_proto_actions_proto_goTypes = nil
	file_proto_actions_proto_depIdxs = nil
}