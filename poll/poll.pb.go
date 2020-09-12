// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.13.0
// source: poll.proto

package poll

import (
	proto "github.com/golang/protobuf/proto"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
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

// A Status records the status of a single repository.
type Status struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Repository string `protobuf:"bytes,1,opt,name=repository,proto3" json:"repository,omitempty"`                    // repository fetch URL
	RefName    string `protobuf:"bytes,2,opt,name=ref_name,json=refName,proto3" json:"ref_name,omitempty"`           // reference name to fetch (default is HEAD)
	Digest     []byte `protobuf:"bytes,3,opt,name=digest,proto3" json:"digest,omitempty"`                            // latest known digest
	ErrorCount int32  `protobuf:"varint,6,opt,name=error_count,json=errorCount,proto3" json:"error_count,omitempty"` // fetch errors since last successful check
	Prefix     string `protobuf:"bytes,7,opt,name=prefix,proto3" json:"prefix,omitempty"`                            // package prefix to attribute to this repository
	Tag        string `protobuf:"bytes,8,opt,name=tag,proto3" json:"tag,omitempty"`                                  // storage tag for this status
	// When the last check of this repository was made. If unset, the repository
	// has never been checked.
	LastCheck *timestamp.Timestamp `protobuf:"bytes,4,opt,name=last_check,json=lastCheck,proto3" json:"last_check,omitempty"`
	// The history of updates, oldest to newest.
	Updates []*Status_Update `protobuf:"bytes,5,rep,name=updates,proto3" json:"updates,omitempty"`
}

func (x *Status) Reset() {
	*x = Status{}
	if protoimpl.UnsafeEnabled {
		mi := &file_poll_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Status) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Status) ProtoMessage() {}

func (x *Status) ProtoReflect() protoreflect.Message {
	mi := &file_poll_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Status.ProtoReflect.Descriptor instead.
func (*Status) Descriptor() ([]byte, []int) {
	return file_poll_proto_rawDescGZIP(), []int{0}
}

func (x *Status) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *Status) GetRefName() string {
	if x != nil {
		return x.RefName
	}
	return ""
}

func (x *Status) GetDigest() []byte {
	if x != nil {
		return x.Digest
	}
	return nil
}

func (x *Status) GetErrorCount() int32 {
	if x != nil {
		return x.ErrorCount
	}
	return 0
}

func (x *Status) GetPrefix() string {
	if x != nil {
		return x.Prefix
	}
	return ""
}

func (x *Status) GetTag() string {
	if x != nil {
		return x.Tag
	}
	return ""
}

func (x *Status) GetLastCheck() *timestamp.Timestamp {
	if x != nil {
		return x.LastCheck
	}
	return nil
}

func (x *Status) GetUpdates() []*Status_Update {
	if x != nil {
		return x.Updates
	}
	return nil
}

type Status_Update struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// When this update was discovered.
	When *timestamp.Timestamp `protobuf:"bytes,1,opt,name=when,proto3" json:"when,omitempty"`
	// The digest at the time of this update.
	Digest []byte `protobuf:"bytes,2,opt,name=digest,proto3" json:"digest,omitempty"`
}

func (x *Status_Update) Reset() {
	*x = Status_Update{}
	if protoimpl.UnsafeEnabled {
		mi := &file_poll_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Status_Update) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Status_Update) ProtoMessage() {}

func (x *Status_Update) ProtoReflect() protoreflect.Message {
	mi := &file_poll_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Status_Update.ProtoReflect.Descriptor instead.
func (*Status_Update) Descriptor() ([]byte, []int) {
	return file_poll_proto_rawDescGZIP(), []int{0, 0}
}

func (x *Status_Update) GetWhen() *timestamp.Timestamp {
	if x != nil {
		return x.When
	}
	return nil
}

func (x *Status_Update) GetDigest() []byte {
	if x != nil {
		return x.Digest
	}
	return nil
}

var File_poll_proto protoreflect.FileDescriptor

var file_poll_proto_rawDesc = []byte{
	0x0a, 0x0a, 0x70, 0x6f, 0x6c, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x04, 0x70, 0x6f,
	0x6c, 0x6c, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0xe2, 0x02, 0x0a, 0x06, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x1e,
	0x0a, 0x0a, 0x72, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x0a, 0x72, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x12, 0x19,
	0x0a, 0x08, 0x72, 0x65, 0x66, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x07, 0x72, 0x65, 0x66, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x64, 0x69, 0x67,
	0x65, 0x73, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x06, 0x64, 0x69, 0x67, 0x65, 0x73,
	0x74, 0x12, 0x1f, 0x0a, 0x0b, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x5f, 0x63, 0x6f, 0x75, 0x6e, 0x74,
	0x18, 0x06, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0a, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x43, 0x6f, 0x75,
	0x6e, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x70, 0x72, 0x65, 0x66, 0x69, 0x78, 0x18, 0x07, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x06, 0x70, 0x72, 0x65, 0x66, 0x69, 0x78, 0x12, 0x10, 0x0a, 0x03, 0x74, 0x61,
	0x67, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x74, 0x61, 0x67, 0x12, 0x39, 0x0a, 0x0a,
	0x6c, 0x61, 0x73, 0x74, 0x5f, 0x63, 0x68, 0x65, 0x63, 0x6b, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x6c, 0x61,
	0x73, 0x74, 0x43, 0x68, 0x65, 0x63, 0x6b, 0x12, 0x2d, 0x0a, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74,
	0x65, 0x73, 0x18, 0x05, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x70, 0x6f, 0x6c, 0x6c, 0x2e,
	0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x52, 0x07, 0x75,
	0x70, 0x64, 0x61, 0x74, 0x65, 0x73, 0x1a, 0x50, 0x0a, 0x06, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65,
	0x12, 0x2e, 0x0a, 0x04, 0x77, 0x68, 0x65, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x04, 0x77, 0x68, 0x65, 0x6e,
	0x12, 0x16, 0x0a, 0x06, 0x64, 0x69, 0x67, 0x65, 0x73, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c,
	0x52, 0x06, 0x64, 0x69, 0x67, 0x65, 0x73, 0x74, 0x42, 0x08, 0x5a, 0x06, 0x2e, 0x3b, 0x70, 0x6f,
	0x6c, 0x6c, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_poll_proto_rawDescOnce sync.Once
	file_poll_proto_rawDescData = file_poll_proto_rawDesc
)

func file_poll_proto_rawDescGZIP() []byte {
	file_poll_proto_rawDescOnce.Do(func() {
		file_poll_proto_rawDescData = protoimpl.X.CompressGZIP(file_poll_proto_rawDescData)
	})
	return file_poll_proto_rawDescData
}

var file_poll_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_poll_proto_goTypes = []interface{}{
	(*Status)(nil),              // 0: poll.Status
	(*Status_Update)(nil),       // 1: poll.Status.Update
	(*timestamp.Timestamp)(nil), // 2: google.protobuf.Timestamp
}
var file_poll_proto_depIdxs = []int32{
	2, // 0: poll.Status.last_check:type_name -> google.protobuf.Timestamp
	1, // 1: poll.Status.updates:type_name -> poll.Status.Update
	2, // 2: poll.Status.Update.when:type_name -> google.protobuf.Timestamp
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_poll_proto_init() }
func file_poll_proto_init() {
	if File_poll_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_poll_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Status); i {
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
		file_poll_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Status_Update); i {
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
			RawDescriptor: file_poll_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_poll_proto_goTypes,
		DependencyIndexes: file_poll_proto_depIdxs,
		MessageInfos:      file_poll_proto_msgTypes,
	}.Build()
	File_poll_proto = out.File
	file_poll_proto_rawDesc = nil
	file_poll_proto_goTypes = nil
	file_poll_proto_depIdxs = nil
}
