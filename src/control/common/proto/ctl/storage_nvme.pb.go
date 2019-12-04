// Code generated by protoc-gen-go. DO NOT EDIT.
// source: storage_nvme.proto

package ctl

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// NvmeController represents an NVMe Controller (SSD).
type NvmeController struct {
	Model                string                      `protobuf:"bytes,1,opt,name=model,proto3" json:"model,omitempty"`
	Serial               string                      `protobuf:"bytes,2,opt,name=serial,proto3" json:"serial,omitempty"`
	Pciaddr              string                      `protobuf:"bytes,3,opt,name=pciaddr,proto3" json:"pciaddr,omitempty"`
	Fwrev                string                      `protobuf:"bytes,4,opt,name=fwrev,proto3" json:"fwrev,omitempty"`
	Socketid             int32                       `protobuf:"varint,5,opt,name=socketid,proto3" json:"socketid,omitempty"`
	Healthstats          *NvmeController_Health      `protobuf:"bytes,6,opt,name=healthstats,proto3" json:"healthstats,omitempty"`
	Namespaces           []*NvmeController_Namespace `protobuf:"bytes,7,rep,name=namespaces,proto3" json:"namespaces,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                    `json:"-"`
	XXX_unrecognized     []byte                      `json:"-"`
	XXX_sizecache        int32                       `json:"-"`
}

func (m *NvmeController) Reset()         { *m = NvmeController{} }
func (m *NvmeController) String() string { return proto.CompactTextString(m) }
func (*NvmeController) ProtoMessage()    {}
func (*NvmeController) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_nvme_d6a1f773b02577a4, []int{0}
}
func (m *NvmeController) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NvmeController.Unmarshal(m, b)
}
func (m *NvmeController) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NvmeController.Marshal(b, m, deterministic)
}
func (dst *NvmeController) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NvmeController.Merge(dst, src)
}
func (m *NvmeController) XXX_Size() int {
	return xxx_messageInfo_NvmeController.Size(m)
}
func (m *NvmeController) XXX_DiscardUnknown() {
	xxx_messageInfo_NvmeController.DiscardUnknown(m)
}

var xxx_messageInfo_NvmeController proto.InternalMessageInfo

func (m *NvmeController) GetModel() string {
	if m != nil {
		return m.Model
	}
	return ""
}

func (m *NvmeController) GetSerial() string {
	if m != nil {
		return m.Serial
	}
	return ""
}

func (m *NvmeController) GetPciaddr() string {
	if m != nil {
		return m.Pciaddr
	}
	return ""
}

func (m *NvmeController) GetFwrev() string {
	if m != nil {
		return m.Fwrev
	}
	return ""
}

func (m *NvmeController) GetSocketid() int32 {
	if m != nil {
		return m.Socketid
	}
	return 0
}

func (m *NvmeController) GetHealthstats() *NvmeController_Health {
	if m != nil {
		return m.Healthstats
	}
	return nil
}

func (m *NvmeController) GetNamespaces() []*NvmeController_Namespace {
	if m != nil {
		return m.Namespaces
	}
	return nil
}

// Namespace represents a namespace created on an NvmeController.
type NvmeController_Namespace struct {
	Id                   int32    `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	Size                 int32    `protobuf:"varint,2,opt,name=size,proto3" json:"size,omitempty"`
	Ctrlrpciaddr         string   `protobuf:"bytes,3,opt,name=ctrlrpciaddr,proto3" json:"ctrlrpciaddr,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NvmeController_Namespace) Reset()         { *m = NvmeController_Namespace{} }
func (m *NvmeController_Namespace) String() string { return proto.CompactTextString(m) }
func (*NvmeController_Namespace) ProtoMessage()    {}
func (*NvmeController_Namespace) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_nvme_d6a1f773b02577a4, []int{0, 0}
}
func (m *NvmeController_Namespace) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NvmeController_Namespace.Unmarshal(m, b)
}
func (m *NvmeController_Namespace) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NvmeController_Namespace.Marshal(b, m, deterministic)
}
func (dst *NvmeController_Namespace) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NvmeController_Namespace.Merge(dst, src)
}
func (m *NvmeController_Namespace) XXX_Size() int {
	return xxx_messageInfo_NvmeController_Namespace.Size(m)
}
func (m *NvmeController_Namespace) XXX_DiscardUnknown() {
	xxx_messageInfo_NvmeController_Namespace.DiscardUnknown(m)
}

var xxx_messageInfo_NvmeController_Namespace proto.InternalMessageInfo

func (m *NvmeController_Namespace) GetId() int32 {
	if m != nil {
		return m.Id
	}
	return 0
}

func (m *NvmeController_Namespace) GetSize() int32 {
	if m != nil {
		return m.Size
	}
	return 0
}

func (m *NvmeController_Namespace) GetCtrlrpciaddr() string {
	if m != nil {
		return m.Ctrlrpciaddr
	}
	return ""
}

type NvmeController_Health struct {
	Temp            uint32 `protobuf:"varint,1,opt,name=temp,proto3" json:"temp,omitempty"`
	Tempwarntime    uint32 `protobuf:"varint,2,opt,name=tempwarntime,proto3" json:"tempwarntime,omitempty"`
	Tempcrittime    uint32 `protobuf:"varint,3,opt,name=tempcrittime,proto3" json:"tempcrittime,omitempty"`
	Ctrlbusytime    uint64 `protobuf:"varint,4,opt,name=ctrlbusytime,proto3" json:"ctrlbusytime,omitempty"`
	Powercycles     uint64 `protobuf:"varint,5,opt,name=powercycles,proto3" json:"powercycles,omitempty"`
	Poweronhours    uint64 `protobuf:"varint,6,opt,name=poweronhours,proto3" json:"poweronhours,omitempty"`
	Unsafeshutdowns uint64 `protobuf:"varint,7,opt,name=unsafeshutdowns,proto3" json:"unsafeshutdowns,omitempty"`
	Mediaerrors     uint64 `protobuf:"varint,8,opt,name=mediaerrors,proto3" json:"mediaerrors,omitempty"`
	Errorlogentries uint64 `protobuf:"varint,9,opt,name=errorlogentries,proto3" json:"errorlogentries,omitempty"`
	// critical warnings
	Tempwarn             bool     `protobuf:"varint,10,opt,name=tempwarn,proto3" json:"tempwarn,omitempty"`
	Availsparewarn       bool     `protobuf:"varint,11,opt,name=availsparewarn,proto3" json:"availsparewarn,omitempty"`
	Reliabilitywarn      bool     `protobuf:"varint,12,opt,name=reliabilitywarn,proto3" json:"reliabilitywarn,omitempty"`
	Readonlywarn         bool     `protobuf:"varint,13,opt,name=readonlywarn,proto3" json:"readonlywarn,omitempty"`
	Volatilewarn         bool     `protobuf:"varint,14,opt,name=volatilewarn,proto3" json:"volatilewarn,omitempty"`
	Ctrlrpciaddr         string   `protobuf:"bytes,15,opt,name=ctrlrpciaddr,proto3" json:"ctrlrpciaddr,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NvmeController_Health) Reset()         { *m = NvmeController_Health{} }
func (m *NvmeController_Health) String() string { return proto.CompactTextString(m) }
func (*NvmeController_Health) ProtoMessage()    {}
func (*NvmeController_Health) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_nvme_d6a1f773b02577a4, []int{0, 1}
}
func (m *NvmeController_Health) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NvmeController_Health.Unmarshal(m, b)
}
func (m *NvmeController_Health) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NvmeController_Health.Marshal(b, m, deterministic)
}
func (dst *NvmeController_Health) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NvmeController_Health.Merge(dst, src)
}
func (m *NvmeController_Health) XXX_Size() int {
	return xxx_messageInfo_NvmeController_Health.Size(m)
}
func (m *NvmeController_Health) XXX_DiscardUnknown() {
	xxx_messageInfo_NvmeController_Health.DiscardUnknown(m)
}

var xxx_messageInfo_NvmeController_Health proto.InternalMessageInfo

func (m *NvmeController_Health) GetTemp() uint32 {
	if m != nil {
		return m.Temp
	}
	return 0
}

func (m *NvmeController_Health) GetTempwarntime() uint32 {
	if m != nil {
		return m.Tempwarntime
	}
	return 0
}

func (m *NvmeController_Health) GetTempcrittime() uint32 {
	if m != nil {
		return m.Tempcrittime
	}
	return 0
}

func (m *NvmeController_Health) GetCtrlbusytime() uint64 {
	if m != nil {
		return m.Ctrlbusytime
	}
	return 0
}

func (m *NvmeController_Health) GetPowercycles() uint64 {
	if m != nil {
		return m.Powercycles
	}
	return 0
}

func (m *NvmeController_Health) GetPoweronhours() uint64 {
	if m != nil {
		return m.Poweronhours
	}
	return 0
}

func (m *NvmeController_Health) GetUnsafeshutdowns() uint64 {
	if m != nil {
		return m.Unsafeshutdowns
	}
	return 0
}

func (m *NvmeController_Health) GetMediaerrors() uint64 {
	if m != nil {
		return m.Mediaerrors
	}
	return 0
}

func (m *NvmeController_Health) GetErrorlogentries() uint64 {
	if m != nil {
		return m.Errorlogentries
	}
	return 0
}

func (m *NvmeController_Health) GetTempwarn() bool {
	if m != nil {
		return m.Tempwarn
	}
	return false
}

func (m *NvmeController_Health) GetAvailsparewarn() bool {
	if m != nil {
		return m.Availsparewarn
	}
	return false
}

func (m *NvmeController_Health) GetReliabilitywarn() bool {
	if m != nil {
		return m.Reliabilitywarn
	}
	return false
}

func (m *NvmeController_Health) GetReadonlywarn() bool {
	if m != nil {
		return m.Readonlywarn
	}
	return false
}

func (m *NvmeController_Health) GetVolatilewarn() bool {
	if m != nil {
		return m.Volatilewarn
	}
	return false
}

func (m *NvmeController_Health) GetCtrlrpciaddr() string {
	if m != nil {
		return m.Ctrlrpciaddr
	}
	return ""
}

// NvmeControllerResult represents state of operation performed on controller.
type NvmeControllerResult struct {
	Pciaddr              string         `protobuf:"bytes,1,opt,name=pciaddr,proto3" json:"pciaddr,omitempty"`
	State                *ResponseState `protobuf:"bytes,2,opt,name=state,proto3" json:"state,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *NvmeControllerResult) Reset()         { *m = NvmeControllerResult{} }
func (m *NvmeControllerResult) String() string { return proto.CompactTextString(m) }
func (*NvmeControllerResult) ProtoMessage()    {}
func (*NvmeControllerResult) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_nvme_d6a1f773b02577a4, []int{1}
}
func (m *NvmeControllerResult) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NvmeControllerResult.Unmarshal(m, b)
}
func (m *NvmeControllerResult) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NvmeControllerResult.Marshal(b, m, deterministic)
}
func (dst *NvmeControllerResult) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NvmeControllerResult.Merge(dst, src)
}
func (m *NvmeControllerResult) XXX_Size() int {
	return xxx_messageInfo_NvmeControllerResult.Size(m)
}
func (m *NvmeControllerResult) XXX_DiscardUnknown() {
	xxx_messageInfo_NvmeControllerResult.DiscardUnknown(m)
}

var xxx_messageInfo_NvmeControllerResult proto.InternalMessageInfo

func (m *NvmeControllerResult) GetPciaddr() string {
	if m != nil {
		return m.Pciaddr
	}
	return ""
}

func (m *NvmeControllerResult) GetState() *ResponseState {
	if m != nil {
		return m.State
	}
	return nil
}

type PrepareNvmeReq struct {
	Pciwhitelist         string   `protobuf:"bytes,1,opt,name=pciwhitelist,proto3" json:"pciwhitelist,omitempty"`
	Nrhugepages          int32    `protobuf:"varint,2,opt,name=nrhugepages,proto3" json:"nrhugepages,omitempty"`
	Targetuser           string   `protobuf:"bytes,3,opt,name=targetuser,proto3" json:"targetuser,omitempty"`
	Reset_               bool     `protobuf:"varint,4,opt,name=reset,proto3" json:"reset,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PrepareNvmeReq) Reset()         { *m = PrepareNvmeReq{} }
func (m *PrepareNvmeReq) String() string { return proto.CompactTextString(m) }
func (*PrepareNvmeReq) ProtoMessage()    {}
func (*PrepareNvmeReq) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_nvme_d6a1f773b02577a4, []int{2}
}
func (m *PrepareNvmeReq) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PrepareNvmeReq.Unmarshal(m, b)
}
func (m *PrepareNvmeReq) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PrepareNvmeReq.Marshal(b, m, deterministic)
}
func (dst *PrepareNvmeReq) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PrepareNvmeReq.Merge(dst, src)
}
func (m *PrepareNvmeReq) XXX_Size() int {
	return xxx_messageInfo_PrepareNvmeReq.Size(m)
}
func (m *PrepareNvmeReq) XXX_DiscardUnknown() {
	xxx_messageInfo_PrepareNvmeReq.DiscardUnknown(m)
}

var xxx_messageInfo_PrepareNvmeReq proto.InternalMessageInfo

func (m *PrepareNvmeReq) GetPciwhitelist() string {
	if m != nil {
		return m.Pciwhitelist
	}
	return ""
}

func (m *PrepareNvmeReq) GetNrhugepages() int32 {
	if m != nil {
		return m.Nrhugepages
	}
	return 0
}

func (m *PrepareNvmeReq) GetTargetuser() string {
	if m != nil {
		return m.Targetuser
	}
	return ""
}

func (m *PrepareNvmeReq) GetReset_() bool {
	if m != nil {
		return m.Reset_
	}
	return false
}

type PrepareNvmeResp struct {
	State                *ResponseState `protobuf:"bytes,1,opt,name=state,proto3" json:"state,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *PrepareNvmeResp) Reset()         { *m = PrepareNvmeResp{} }
func (m *PrepareNvmeResp) String() string { return proto.CompactTextString(m) }
func (*PrepareNvmeResp) ProtoMessage()    {}
func (*PrepareNvmeResp) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_nvme_d6a1f773b02577a4, []int{3}
}
func (m *PrepareNvmeResp) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PrepareNvmeResp.Unmarshal(m, b)
}
func (m *PrepareNvmeResp) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PrepareNvmeResp.Marshal(b, m, deterministic)
}
func (dst *PrepareNvmeResp) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PrepareNvmeResp.Merge(dst, src)
}
func (m *PrepareNvmeResp) XXX_Size() int {
	return xxx_messageInfo_PrepareNvmeResp.Size(m)
}
func (m *PrepareNvmeResp) XXX_DiscardUnknown() {
	xxx_messageInfo_PrepareNvmeResp.DiscardUnknown(m)
}

var xxx_messageInfo_PrepareNvmeResp proto.InternalMessageInfo

func (m *PrepareNvmeResp) GetState() *ResponseState {
	if m != nil {
		return m.State
	}
	return nil
}

type ScanNvmeReq struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ScanNvmeReq) Reset()         { *m = ScanNvmeReq{} }
func (m *ScanNvmeReq) String() string { return proto.CompactTextString(m) }
func (*ScanNvmeReq) ProtoMessage()    {}
func (*ScanNvmeReq) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_nvme_d6a1f773b02577a4, []int{4}
}
func (m *ScanNvmeReq) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ScanNvmeReq.Unmarshal(m, b)
}
func (m *ScanNvmeReq) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ScanNvmeReq.Marshal(b, m, deterministic)
}
func (dst *ScanNvmeReq) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ScanNvmeReq.Merge(dst, src)
}
func (m *ScanNvmeReq) XXX_Size() int {
	return xxx_messageInfo_ScanNvmeReq.Size(m)
}
func (m *ScanNvmeReq) XXX_DiscardUnknown() {
	xxx_messageInfo_ScanNvmeReq.DiscardUnknown(m)
}

var xxx_messageInfo_ScanNvmeReq proto.InternalMessageInfo

type ScanNvmeResp struct {
	Ctrlrs               []*NvmeController `protobuf:"bytes,1,rep,name=ctrlrs,proto3" json:"ctrlrs,omitempty"`
	State                *ResponseState    `protobuf:"bytes,2,opt,name=state,proto3" json:"state,omitempty"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}

func (m *ScanNvmeResp) Reset()         { *m = ScanNvmeResp{} }
func (m *ScanNvmeResp) String() string { return proto.CompactTextString(m) }
func (*ScanNvmeResp) ProtoMessage()    {}
func (*ScanNvmeResp) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_nvme_d6a1f773b02577a4, []int{5}
}
func (m *ScanNvmeResp) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ScanNvmeResp.Unmarshal(m, b)
}
func (m *ScanNvmeResp) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ScanNvmeResp.Marshal(b, m, deterministic)
}
func (dst *ScanNvmeResp) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ScanNvmeResp.Merge(dst, src)
}
func (m *ScanNvmeResp) XXX_Size() int {
	return xxx_messageInfo_ScanNvmeResp.Size(m)
}
func (m *ScanNvmeResp) XXX_DiscardUnknown() {
	xxx_messageInfo_ScanNvmeResp.DiscardUnknown(m)
}

var xxx_messageInfo_ScanNvmeResp proto.InternalMessageInfo

func (m *ScanNvmeResp) GetCtrlrs() []*NvmeController {
	if m != nil {
		return m.Ctrlrs
	}
	return nil
}

func (m *ScanNvmeResp) GetState() *ResponseState {
	if m != nil {
		return m.State
	}
	return nil
}

type FormatNvmeReq struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *FormatNvmeReq) Reset()         { *m = FormatNvmeReq{} }
func (m *FormatNvmeReq) String() string { return proto.CompactTextString(m) }
func (*FormatNvmeReq) ProtoMessage()    {}
func (*FormatNvmeReq) Descriptor() ([]byte, []int) {
	return fileDescriptor_storage_nvme_d6a1f773b02577a4, []int{6}
}
func (m *FormatNvmeReq) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_FormatNvmeReq.Unmarshal(m, b)
}
func (m *FormatNvmeReq) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_FormatNvmeReq.Marshal(b, m, deterministic)
}
func (dst *FormatNvmeReq) XXX_Merge(src proto.Message) {
	xxx_messageInfo_FormatNvmeReq.Merge(dst, src)
}
func (m *FormatNvmeReq) XXX_Size() int {
	return xxx_messageInfo_FormatNvmeReq.Size(m)
}
func (m *FormatNvmeReq) XXX_DiscardUnknown() {
	xxx_messageInfo_FormatNvmeReq.DiscardUnknown(m)
}

var xxx_messageInfo_FormatNvmeReq proto.InternalMessageInfo

func init() {
	proto.RegisterType((*NvmeController)(nil), "ctl.NvmeController")
	proto.RegisterType((*NvmeController_Namespace)(nil), "ctl.NvmeController.Namespace")
	proto.RegisterType((*NvmeController_Health)(nil), "ctl.NvmeController.Health")
	proto.RegisterType((*NvmeControllerResult)(nil), "ctl.NvmeControllerResult")
	proto.RegisterType((*PrepareNvmeReq)(nil), "ctl.PrepareNvmeReq")
	proto.RegisterType((*PrepareNvmeResp)(nil), "ctl.PrepareNvmeResp")
	proto.RegisterType((*ScanNvmeReq)(nil), "ctl.ScanNvmeReq")
	proto.RegisterType((*ScanNvmeResp)(nil), "ctl.ScanNvmeResp")
	proto.RegisterType((*FormatNvmeReq)(nil), "ctl.FormatNvmeReq")
}

func init() { proto.RegisterFile("storage_nvme.proto", fileDescriptor_storage_nvme_d6a1f773b02577a4) }

var fileDescriptor_storage_nvme_d6a1f773b02577a4 = []byte{
	// 629 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x54, 0x4f, 0x6b, 0xdb, 0x4e,
	0x10, 0x45, 0xf1, 0x9f, 0x38, 0x23, 0xff, 0x81, 0xfd, 0x85, 0x1f, 0xc2, 0xd0, 0x62, 0x7c, 0x28,
	0x82, 0x82, 0x0f, 0xe9, 0xb1, 0xed, 0xa9, 0x50, 0x7a, 0x0a, 0x65, 0x73, 0xeb, 0xa5, 0x6c, 0xa4,
	0x89, 0xbd, 0x74, 0xb5, 0xab, 0xee, 0xae, 0x6c, 0xd2, 0xcf, 0xd0, 0xcf, 0xd0, 0x6f, 0x5a, 0x28,
	0x3b, 0x6b, 0x25, 0x92, 0x13, 0x28, 0x3d, 0x59, 0xf3, 0xf6, 0xe9, 0xcd, 0xce, 0xd3, 0x1b, 0x03,
	0x73, 0xde, 0x58, 0xb1, 0xc5, 0xaf, 0x7a, 0x5f, 0xe1, 0xa6, 0xb6, 0xc6, 0x1b, 0x36, 0x28, 0xbc,
	0x5a, 0x4e, 0x0b, 0x53, 0x55, 0x46, 0x47, 0x68, 0xfd, 0x7b, 0x0c, 0xf3, 0xeb, 0x7d, 0x85, 0x1f,
	0x8c, 0xf6, 0xd6, 0x28, 0x85, 0x96, 0x5d, 0xc2, 0xa8, 0x32, 0x25, 0xaa, 0x2c, 0x59, 0x25, 0xf9,
	0x05, 0x8f, 0x05, 0xfb, 0x1f, 0xc6, 0x0e, 0xad, 0x14, 0x2a, 0x3b, 0x23, 0xf8, 0x58, 0xb1, 0x0c,
	0xce, 0xeb, 0x42, 0x8a, 0xb2, 0xb4, 0xd9, 0x80, 0x0e, 0xda, 0x32, 0xe8, 0xdc, 0x1d, 0x2c, 0xee,
	0xb3, 0x61, 0xd4, 0xa1, 0x82, 0x2d, 0x61, 0xe2, 0x4c, 0xf1, 0x0d, 0xbd, 0x2c, 0xb3, 0xd1, 0x2a,
	0xc9, 0x47, 0xfc, 0xa1, 0x66, 0xef, 0x20, 0xdd, 0xa1, 0x50, 0x7e, 0xe7, 0xbc, 0xf0, 0x2e, 0x1b,
	0xaf, 0x92, 0x3c, 0xbd, 0x5a, 0x6e, 0x0a, 0xaf, 0x36, 0xfd, 0x3b, 0x6e, 0x3e, 0x11, 0x8d, 0x77,
	0xe9, 0xec, 0x3d, 0x80, 0x16, 0x15, 0xba, 0x5a, 0x14, 0xe8, 0xb2, 0xf3, 0xd5, 0x20, 0x4f, 0xaf,
	0x5e, 0x3c, 0xf7, 0xf2, 0x75, 0xcb, 0xe2, 0x9d, 0x17, 0x96, 0x37, 0x70, 0xf1, 0x70, 0xc0, 0xe6,
	0x70, 0x26, 0x4b, 0x32, 0x60, 0xc4, 0xcf, 0x64, 0xc9, 0x18, 0x0c, 0x9d, 0xfc, 0x81, 0x34, 0xfb,
	0x88, 0xd3, 0x33, 0x5b, 0xc3, 0xb4, 0xf0, 0x56, 0xd9, 0xfe, 0xf8, 0x3d, 0x6c, 0xf9, 0x6b, 0x08,
	0xe3, 0x78, 0xd7, 0x20, 0xe1, 0xb1, 0xaa, 0x49, 0x74, 0xc6, 0xe9, 0x39, 0x48, 0x84, 0xdf, 0x83,
	0xb0, 0xda, 0xcb, 0x2a, 0xca, 0xcf, 0x78, 0x0f, 0x6b, 0x39, 0x85, 0x95, 0x9e, 0x38, 0x83, 0x47,
	0x4e, 0x8b, 0xb5, 0x57, 0xb9, 0x6d, 0xdc, 0x3d, 0x71, 0x82, 0xe3, 0x43, 0xde, 0xc3, 0xd8, 0x0a,
	0xd2, 0xda, 0x1c, 0xd0, 0x16, 0xf7, 0x85, 0x42, 0x47, 0xde, 0x0f, 0x79, 0x17, 0x0a, 0x2a, 0x54,
	0x1a, 0xbd, 0x33, 0x8d, 0x8d, 0xfe, 0x0f, 0x79, 0x0f, 0x63, 0x39, 0x2c, 0x1a, 0xed, 0xc4, 0x1d,
	0xba, 0x5d, 0xe3, 0x4b, 0x73, 0xd0, 0xc1, 0xe9, 0x40, 0x3b, 0x85, 0x43, 0xbf, 0x0a, 0x4b, 0x29,
	0xd0, 0x5a, 0x63, 0x5d, 0x36, 0x89, 0xfd, 0x3a, 0x50, 0xd0, 0xa2, 0x27, 0x65, 0xb6, 0xa8, 0xbd,
	0x95, 0xe8, 0xb2, 0x8b, 0xa8, 0x75, 0x02, 0x87, 0xd0, 0xb4, 0x9e, 0x64, 0xb0, 0x4a, 0xf2, 0x09,
	0x7f, 0xa8, 0xd9, 0x2b, 0x98, 0x8b, 0xbd, 0x90, 0xca, 0xd5, 0xc2, 0x22, 0x31, 0x52, 0x62, 0x9c,
	0xa0, 0xa1, 0x9b, 0x45, 0x25, 0xc5, 0xad, 0x54, 0xd2, 0xdf, 0x13, 0x71, 0x4a, 0xc4, 0x53, 0x38,
	0xf8, 0x60, 0x51, 0x94, 0x46, 0xab, 0x48, 0x9b, 0x11, 0xad, 0x87, 0x05, 0xce, 0xde, 0x28, 0xe1,
	0xa5, 0x8a, 0x3d, 0xe7, 0x91, 0xd3, 0xc5, 0x9e, 0x04, 0x64, 0xf1, 0x34, 0x20, 0xeb, 0x2f, 0x70,
	0xd9, 0x4f, 0x27, 0x47, 0xd7, 0x28, 0xdf, 0x5d, 0xab, 0xa4, 0xbf, 0x56, 0x39, 0x8c, 0x42, 0xde,
	0x63, 0x58, 0xd2, 0x2b, 0x46, 0x09, 0xe7, 0xe8, 0x6a, 0xa3, 0x1d, 0xde, 0x84, 0x13, 0x1e, 0x09,
	0xeb, 0x9f, 0x09, 0xcc, 0x3f, 0x5b, 0x0c, 0x0e, 0x84, 0x1e, 0x1c, 0xbf, 0xd3, 0x27, 0x2e, 0xe4,
	0x61, 0x27, 0x3d, 0x2a, 0xe9, 0xfc, 0x51, 0xbb, 0x87, 0x85, 0x0f, 0xa7, 0xed, 0xae, 0xd9, 0x62,
	0x2d, 0xb6, 0xe8, 0x8e, 0x91, 0xef, 0x42, 0xec, 0x25, 0x80, 0x17, 0x76, 0x8b, 0xbe, 0x71, 0xd8,
	0xe6, 0xbe, 0x83, 0x84, 0xcd, 0xb7, 0xe8, 0xd0, 0x53, 0x0e, 0x27, 0x3c, 0x16, 0xeb, 0xb7, 0xb0,
	0xe8, 0xdd, 0xc6, 0xd5, 0x8f, 0xb3, 0x24, 0x7f, 0x9b, 0x65, 0x06, 0xe9, 0x4d, 0x21, 0xf4, 0x71,
	0x8e, 0x35, 0xc2, 0xf4, 0xb1, 0x74, 0x35, 0x7b, 0x0d, 0x63, 0xb2, 0xd5, 0x65, 0x09, 0xed, 0xfd,
	0x7f, 0xcf, 0xec, 0x3d, 0x3f, 0x52, 0xfe, 0xc1, 0xc1, 0x05, 0xcc, 0x3e, 0x1a, 0x5b, 0x09, 0x7f,
	0xec, 0x7b, 0x3b, 0xa6, 0x7f, 0xcd, 0x37, 0x7f, 0x02, 0x00, 0x00, 0xff, 0xff, 0x68, 0xe3, 0x7e,
	0x4a, 0x5e, 0x05, 0x00, 0x00,
}
