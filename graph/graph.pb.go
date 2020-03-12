// Protocol definitions for dependency graph storage.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.20.1
// 	protoc        v3.11.4
// source: graph.proto

package graph

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

type Row_Type int32

const (
	Row_UNKNOWN Row_Type = 0 // the package type is not known
	Row_STDLIB  Row_Type = 1 // this is a standard library package
	Row_LIBRARY Row_Type = 2 // this is a library package
	Row_PROGRAM Row_Type = 3 // this is an executable
)

// Enum value maps for Row_Type.
var (
	Row_Type_name = map[int32]string{
		0: "UNKNOWN",
		1: "STDLIB",
		2: "LIBRARY",
		3: "PROGRAM",
	}
	Row_Type_value = map[string]int32{
		"UNKNOWN": 0,
		"STDLIB":  1,
		"LIBRARY": 2,
		"PROGRAM": 3,
	}
)

func (x Row_Type) Enum() *Row_Type {
	p := new(Row_Type)
	*p = x
	return p
}

func (x Row_Type) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Row_Type) Descriptor() protoreflect.EnumDescriptor {
	return file_graph_proto_enumTypes[0].Descriptor()
}

func (Row_Type) Type() protoreflect.EnumType {
	return &file_graph_proto_enumTypes[0]
}

func (x Row_Type) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Row_Type.Descriptor instead.
func (Row_Type) EnumDescriptor() ([]byte, []int) {
	return file_graph_proto_rawDescGZIP(), []int{0, 0}
}

// A Row is a single row of the dependency graph adjacency list.
type Row struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The simple name and import path of the package whose row this is.
	Name       string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	ImportPath string `protobuf:"bytes,2,opt,name=import_path,json=importPath,proto3" json:"import_path,omitempty"`
	// The repository where the package was defined.
	Repository string `protobuf:"bytes,3,opt,name=repository,proto3" json:"repository,omitempty"`
	// The import paths of the direct dependencies of source.
	Directs []string `protobuf:"bytes,4,rep,name=directs,proto3" json:"directs,omitempty"`
	// The names and content digests of source files in this package.
	SourceFiles []*Row_File `protobuf:"bytes,5,rep,name=source_files,json=sourceFiles,proto3" json:"source_files,omitempty"`
	// Classify the package type according to its role, if known.
	Type Row_Type `protobuf:"varint,6,opt,name=type,proto3,enum=graph.Row_Type" json:"type,omitempty"`
	// Ranking weight; 0 represents an unranked value.
	Ranking float64 `protobuf:"fixed64,7,opt,name=ranking,proto3" json:"ranking,omitempty"`
}

func (x *Row) Reset() {
	*x = Row{}
	if protoimpl.UnsafeEnabled {
		mi := &file_graph_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Row) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Row) ProtoMessage() {}

func (x *Row) ProtoReflect() protoreflect.Message {
	mi := &file_graph_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Row.ProtoReflect.Descriptor instead.
func (*Row) Descriptor() ([]byte, []int) {
	return file_graph_proto_rawDescGZIP(), []int{0}
}

func (x *Row) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Row) GetImportPath() string {
	if x != nil {
		return x.ImportPath
	}
	return ""
}

func (x *Row) GetRepository() string {
	if x != nil {
		return x.Repository
	}
	return ""
}

func (x *Row) GetDirects() []string {
	if x != nil {
		return x.Directs
	}
	return nil
}

func (x *Row) GetSourceFiles() []*Row_File {
	if x != nil {
		return x.SourceFiles
	}
	return nil
}

func (x *Row) GetType() Row_Type {
	if x != nil {
		return x.Type
	}
	return Row_UNKNOWN
}

func (x *Row) GetRanking() float64 {
	if x != nil {
		return x.Ranking
	}
	return 0
}

type Row_File struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RepoPath string `protobuf:"bytes,1,opt,name=repo_path,json=repoPath,proto3" json:"repo_path,omitempty"` // file path relative to the repository root
	Digest   []byte `protobuf:"bytes,2,opt,name=digest,proto3" json:"digest,omitempty"`                     // content digest (sha256)
}

func (x *Row_File) Reset() {
	*x = Row_File{}
	if protoimpl.UnsafeEnabled {
		mi := &file_graph_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Row_File) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Row_File) ProtoMessage() {}

func (x *Row_File) ProtoReflect() protoreflect.Message {
	mi := &file_graph_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Row_File.ProtoReflect.Descriptor instead.
func (*Row_File) Descriptor() ([]byte, []int) {
	return file_graph_proto_rawDescGZIP(), []int{0, 0}
}

func (x *Row_File) GetRepoPath() string {
	if x != nil {
		return x.RepoPath
	}
	return ""
}

func (x *Row_File) GetDigest() []byte {
	if x != nil {
		return x.Digest
	}
	return nil
}

var File_graph_proto protoreflect.FileDescriptor

var file_graph_proto_rawDesc = []byte{
	0x0a, 0x0b, 0x67, 0x72, 0x61, 0x70, 0x68, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x67,
	0x72, 0x61, 0x70, 0x68, 0x22, 0xdf, 0x02, 0x0a, 0x03, 0x52, 0x6f, 0x77, 0x12, 0x12, 0x0a, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x12, 0x1f, 0x0a, 0x0b, 0x69, 0x6d, 0x70, 0x6f, 0x72, 0x74, 0x5f, 0x70, 0x61, 0x74, 0x68, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x69, 0x6d, 0x70, 0x6f, 0x72, 0x74, 0x50, 0x61, 0x74,
	0x68, 0x12, 0x1e, 0x0a, 0x0a, 0x72, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x72, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72,
	0x79, 0x12, 0x18, 0x0a, 0x07, 0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x73, 0x18, 0x04, 0x20, 0x03,
	0x28, 0x09, 0x52, 0x07, 0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x73, 0x12, 0x32, 0x0a, 0x0c, 0x73,
	0x6f, 0x75, 0x72, 0x63, 0x65, 0x5f, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x18, 0x05, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x0f, 0x2e, 0x67, 0x72, 0x61, 0x70, 0x68, 0x2e, 0x52, 0x6f, 0x77, 0x2e, 0x46, 0x69,
	0x6c, 0x65, 0x52, 0x0b, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x46, 0x69, 0x6c, 0x65, 0x73, 0x12,
	0x23, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x0f, 0x2e,
	0x67, 0x72, 0x61, 0x70, 0x68, 0x2e, 0x52, 0x6f, 0x77, 0x2e, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04,
	0x74, 0x79, 0x70, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x72, 0x61, 0x6e, 0x6b, 0x69, 0x6e, 0x67, 0x18,
	0x07, 0x20, 0x01, 0x28, 0x01, 0x52, 0x07, 0x72, 0x61, 0x6e, 0x6b, 0x69, 0x6e, 0x67, 0x1a, 0x3b,
	0x0a, 0x04, 0x46, 0x69, 0x6c, 0x65, 0x12, 0x1b, 0x0a, 0x09, 0x72, 0x65, 0x70, 0x6f, 0x5f, 0x70,
	0x61, 0x74, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x72, 0x65, 0x70, 0x6f, 0x50,
	0x61, 0x74, 0x68, 0x12, 0x16, 0x0a, 0x06, 0x64, 0x69, 0x67, 0x65, 0x73, 0x74, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x06, 0x64, 0x69, 0x67, 0x65, 0x73, 0x74, 0x22, 0x39, 0x0a, 0x04, 0x54,
	0x79, 0x70, 0x65, 0x12, 0x0b, 0x0a, 0x07, 0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57, 0x4e, 0x10, 0x00,
	0x12, 0x0a, 0x0a, 0x06, 0x53, 0x54, 0x44, 0x4c, 0x49, 0x42, 0x10, 0x01, 0x12, 0x0b, 0x0a, 0x07,
	0x4c, 0x49, 0x42, 0x52, 0x41, 0x52, 0x59, 0x10, 0x02, 0x12, 0x0b, 0x0a, 0x07, 0x50, 0x52, 0x4f,
	0x47, 0x52, 0x41, 0x4d, 0x10, 0x03, 0x42, 0x09, 0x5a, 0x07, 0x2e, 0x3b, 0x67, 0x72, 0x61, 0x70,
	0x68, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_graph_proto_rawDescOnce sync.Once
	file_graph_proto_rawDescData = file_graph_proto_rawDesc
)

func file_graph_proto_rawDescGZIP() []byte {
	file_graph_proto_rawDescOnce.Do(func() {
		file_graph_proto_rawDescData = protoimpl.X.CompressGZIP(file_graph_proto_rawDescData)
	})
	return file_graph_proto_rawDescData
}

var file_graph_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_graph_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_graph_proto_goTypes = []interface{}{
	(Row_Type)(0),    // 0: graph.Row.Type
	(*Row)(nil),      // 1: graph.Row
	(*Row_File)(nil), // 2: graph.Row.File
}
var file_graph_proto_depIdxs = []int32{
	2, // 0: graph.Row.source_files:type_name -> graph.Row.File
	0, // 1: graph.Row.type:type_name -> graph.Row.Type
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_graph_proto_init() }
func file_graph_proto_init() {
	if File_graph_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_graph_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Row); i {
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
		file_graph_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Row_File); i {
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
			RawDescriptor: file_graph_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_graph_proto_goTypes,
		DependencyIndexes: file_graph_proto_depIdxs,
		EnumInfos:         file_graph_proto_enumTypes,
		MessageInfos:      file_graph_proto_msgTypes,
	}.Build()
	File_graph_proto = out.File
	file_graph_proto_rawDesc = nil
	file_graph_proto_goTypes = nil
	file_graph_proto_depIdxs = nil
}
