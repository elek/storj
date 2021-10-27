// Code generated by protoc-gen-go-drpc. DO NOT EDIT.
// protoc-gen-go-drpc version: (devel)
// source: inspector.proto

package internalpb

import (
	bytes "bytes"
	context "context"
	errors "errors"

	jsonpb "github.com/gogo/protobuf/jsonpb"
	proto "github.com/gogo/protobuf/proto"

	drpc "storj.io/drpc"
	drpcerr "storj.io/drpc/drpcerr"
)

type drpcEncoding_File_inspector_proto struct{}

func (drpcEncoding_File_inspector_proto) Marshal(msg drpc.Message) ([]byte, error) {
	return proto.Marshal(msg.(proto.Message))
}

func (drpcEncoding_File_inspector_proto) Unmarshal(buf []byte, msg drpc.Message) error {
	return proto.Unmarshal(buf, msg.(proto.Message))
}

func (drpcEncoding_File_inspector_proto) JSONMarshal(msg drpc.Message) ([]byte, error) {
	var buf bytes.Buffer
	err := new(jsonpb.Marshaler).Marshal(&buf, msg.(proto.Message))
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (drpcEncoding_File_inspector_proto) JSONUnmarshal(buf []byte, msg drpc.Message) error {
	return jsonpb.Unmarshal(bytes.NewReader(buf), msg.(proto.Message))
}

type DRPCHealthInspectorClient interface {
	DRPCConn() drpc.Conn

	ObjectHealth(ctx context.Context, in *ObjectHealthRequest) (*ObjectHealthResponse, error)
	SegmentHealth(ctx context.Context, in *SegmentHealthRequest) (*SegmentHealthResponse, error)
}

type drpcHealthInspectorClient struct {
	cc drpc.Conn
}

func NewDRPCHealthInspectorClient(cc drpc.Conn) DRPCHealthInspectorClient {
	return &drpcHealthInspectorClient{cc}
}

func (c *drpcHealthInspectorClient) DRPCConn() drpc.Conn { return c.cc }

func (c *drpcHealthInspectorClient) ObjectHealth(ctx context.Context, in *ObjectHealthRequest) (*ObjectHealthResponse, error) {
	out := new(ObjectHealthResponse)
	err := c.cc.Invoke(ctx, "/satellite.inspector.HealthInspector/ObjectHealth", drpcEncoding_File_inspector_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *drpcHealthInspectorClient) SegmentHealth(ctx context.Context, in *SegmentHealthRequest) (*SegmentHealthResponse, error) {
	out := new(SegmentHealthResponse)
	err := c.cc.Invoke(ctx, "/satellite.inspector.HealthInspector/SegmentHealth", drpcEncoding_File_inspector_proto{}, in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type DRPCHealthInspectorServer interface {
	ObjectHealth(context.Context, *ObjectHealthRequest) (*ObjectHealthResponse, error)
	SegmentHealth(context.Context, *SegmentHealthRequest) (*SegmentHealthResponse, error)
}

type DRPCHealthInspectorUnimplementedServer struct{}

func (s *DRPCHealthInspectorUnimplementedServer) ObjectHealth(context.Context, *ObjectHealthRequest) (*ObjectHealthResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

func (s *DRPCHealthInspectorUnimplementedServer) SegmentHealth(context.Context, *SegmentHealthRequest) (*SegmentHealthResponse, error) {
	return nil, drpcerr.WithCode(errors.New("Unimplemented"), drpcerr.Unimplemented)
}

type DRPCHealthInspectorDescription struct{}

func (DRPCHealthInspectorDescription) NumMethods() int { return 2 }

func (DRPCHealthInspectorDescription) Method(n int) (string, drpc.Encoding, drpc.Receiver, interface{}, bool) {
	switch n {
	case 0:
		return "/satellite.inspector.HealthInspector/ObjectHealth", drpcEncoding_File_inspector_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCHealthInspectorServer).
					ObjectHealth(
						ctx,
						in1.(*ObjectHealthRequest),
					)
			}, DRPCHealthInspectorServer.ObjectHealth, true
	case 1:
		return "/satellite.inspector.HealthInspector/SegmentHealth", drpcEncoding_File_inspector_proto{},
			func(srv interface{}, ctx context.Context, in1, in2 interface{}) (drpc.Message, error) {
				return srv.(DRPCHealthInspectorServer).
					SegmentHealth(
						ctx,
						in1.(*SegmentHealthRequest),
					)
			}, DRPCHealthInspectorServer.SegmentHealth, true
	default:
		return "", nil, nil, nil, false
	}
}

func DRPCRegisterHealthInspector(mux drpc.Mux, impl DRPCHealthInspectorServer) error {
	return mux.Register(impl, DRPCHealthInspectorDescription{})
}

type DRPCHealthInspector_ObjectHealthStream interface {
	drpc.Stream
	SendAndClose(*ObjectHealthResponse) error
}

type drpcHealthInspector_ObjectHealthStream struct {
	drpc.Stream
}

func (x *drpcHealthInspector_ObjectHealthStream) SendAndClose(m *ObjectHealthResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_inspector_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}

type DRPCHealthInspector_SegmentHealthStream interface {
	drpc.Stream
	SendAndClose(*SegmentHealthResponse) error
}

type drpcHealthInspector_SegmentHealthStream struct {
	drpc.Stream
}

func (x *drpcHealthInspector_SegmentHealthStream) SendAndClose(m *SegmentHealthResponse) error {
	if err := x.MsgSend(m, drpcEncoding_File_inspector_proto{}); err != nil {
		return err
	}
	return x.CloseSend()
}
