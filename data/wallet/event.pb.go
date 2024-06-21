// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.12.4
// source: wallet/event.proto

package wallet

import (
	proto "github.com/golang/protobuf/proto"
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

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type Event struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// the name of the action that it should execute
	Event string `protobuf:"bytes,1,opt,name=Event,proto3" json:"Event,omitempty"`
	// each generated event should be unique
	ID string `protobuf:"bytes,2,opt,name=ID,proto3" json:"ID,omitempty"`
	// the chain or coin that triggered the event
	Chain string `protobuf:"bytes,3,opt,name=Chain,proto3" json:"Chain,omitempty"`
	Coin  string `protobuf:"bytes,4,opt,name=Coin,proto3" json:"Coin,omitempty"`
	// the if of the user associated with the event if any
	UserID uint64 `protobuf:"varint,5,opt,name=UserID,proto3" json:"UserID,omitempty"`
	// metadata
	Meta map[string]string `protobuf:"bytes,6,rep,name=Meta,proto3" json:"Meta,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// data payload of the command
	Payload map[string]string `protobuf:"bytes,7,rep,name=Payload,proto3" json:"Payload,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// external system
	System string `protobuf:"bytes,8,opt,name=System,proto3" json:"System,omitempty"`
}

func (x *Event) Reset() {
	*x = Event{}
	if protoimpl.UnsafeEnabled {
		mi := &file_wallet_event_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Event) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Event) ProtoMessage() {}

func (x *Event) ProtoReflect() protoreflect.Message {
	mi := &file_wallet_event_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Event.ProtoReflect.Descriptor instead.
func (*Event) Descriptor() ([]byte, []int) {
	return file_wallet_event_proto_rawDescGZIP(), []int{0}
}

func (x *Event) GetEvent() string {
	if x != nil {
		return x.Event
	}
	return ""
}

func (x *Event) GetID() string {
	if x != nil {
		return x.ID
	}
	return ""
}

func (x *Event) GetChain() string {
	if x != nil {
		return x.Chain
	}
	return ""
}

func (x *Event) GetCoin() string {
	if x != nil {
		return x.Coin
	}
	return ""
}

func (x *Event) GetUserID() uint64 {
	if x != nil {
		return x.UserID
	}
	return 0
}

func (x *Event) GetMeta() map[string]string {
	if x != nil {
		return x.Meta
	}
	return nil
}

func (x *Event) GetPayload() map[string]string {
	if x != nil {
		return x.Payload
	}
	return nil
}

func (x *Event) GetSystem() string {
	if x != nil {
		return x.System
	}
	return ""
}

var File_wallet_event_proto protoreflect.FileDescriptor

var file_wallet_event_proto_rawDesc = []byte{
	0x0a, 0x12, 0x77, 0x61, 0x6c, 0x6c, 0x65, 0x74, 0x2f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06, 0x77,
	0x61, 0x6c, 0x6c, 0x65, 0x74, 0x22, 0xdf, 0x02, 0x0a, 0x05, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x12,
	0x14, 0x0a, 0x05, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05,
	0x45, 0x76, 0x65, 0x6e, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x02, 0x49, 0x44, 0x12, 0x14, 0x0a, 0x05, 0x43, 0x68, 0x61, 0x69, 0x6e, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x43, 0x68, 0x61, 0x69, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x43,
	0x6f, 0x69, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x43, 0x6f, 0x69, 0x6e, 0x12,
	0x16, 0x0a, 0x06, 0x55, 0x73, 0x65, 0x72, 0x49, 0x44, 0x18, 0x05, 0x20, 0x01, 0x28, 0x04, 0x52,
	0x06, 0x55, 0x73, 0x65, 0x72, 0x49, 0x44, 0x12, 0x2b, 0x0a, 0x04, 0x4d, 0x65, 0x74, 0x61, 0x18,
	0x06, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x77, 0x61, 0x6c, 0x6c, 0x65, 0x74, 0x2e, 0x45,
	0x76, 0x65, 0x6e, 0x74, 0x2e, 0x4d, 0x65, 0x74, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x04,
	0x4d, 0x65, 0x74, 0x61, 0x12, 0x34, 0x0a, 0x07, 0x50, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x18,
	0x07, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x77, 0x61, 0x6c, 0x6c, 0x65, 0x74, 0x2e, 0x45,
	0x76, 0x65, 0x6e, 0x74, 0x2e, 0x50, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x45, 0x6e, 0x74, 0x72,
	0x79, 0x52, 0x07, 0x50, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x12, 0x16, 0x0a, 0x06, 0x53, 0x79,
	0x73, 0x74, 0x65, 0x6d, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x53, 0x79, 0x73, 0x74,
	0x65, 0x6d, 0x1a, 0x37, 0x0a, 0x09, 0x4d, 0x65, 0x74, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12,
	0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65,
	0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x1a, 0x3a, 0x0a, 0x0c, 0x50,
	0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b,
	0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a,
	0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_wallet_event_proto_rawDescOnce sync.Once
	file_wallet_event_proto_rawDescData = file_wallet_event_proto_rawDesc
)

func file_wallet_event_proto_rawDescGZIP() []byte {
	file_wallet_event_proto_rawDescOnce.Do(func() {
		file_wallet_event_proto_rawDescData = protoimpl.X.CompressGZIP(file_wallet_event_proto_rawDescData)
	})
	return file_wallet_event_proto_rawDescData
}

var file_wallet_event_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_wallet_event_proto_goTypes = []interface{}{
	(*Event)(nil), // 0: wallet.Event
	nil,           // 1: wallet.Event.MetaEntry
	nil,           // 2: wallet.Event.PayloadEntry
}
var file_wallet_event_proto_depIdxs = []int32{
	1, // 0: wallet.Event.Meta:type_name -> wallet.Event.MetaEntry
	2, // 1: wallet.Event.Payload:type_name -> wallet.Event.PayloadEntry
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_wallet_event_proto_init() }
func file_wallet_event_proto_init() {
	if File_wallet_event_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_wallet_event_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Event); i {
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
			RawDescriptor: file_wallet_event_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_wallet_event_proto_goTypes,
		DependencyIndexes: file_wallet_event_proto_depIdxs,
		MessageInfos:      file_wallet_event_proto_msgTypes,
	}.Build()
	File_wallet_event_proto = out.File
	file_wallet_event_proto_rawDesc = nil
	file_wallet_event_proto_goTypes = nil
	file_wallet_event_proto_depIdxs = nil
}
