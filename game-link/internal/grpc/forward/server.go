package forward

import (
	"context"
	"log"
	"strconv"

	svcerr "internal/errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	gatepb "internal.proto/pb/gate"
)

// Server implements GateService (Request/Notify/Trigger/Fetch) to accept forwarded gatepb.Pack.
type Server struct {
	gatepb.UnimplementedGateServiceServer
	selfSvt string // 本服務名稱，用於 default handler log
}

// NewServer creates a new Forward server. selfSvt 為本服務名稱（如 "gate"、"auth"），未註冊 method 時 default handler 會寫 log。
func NewServer(selfSvt string) *Server {
	return &Server{selfSvt: selfSvt}
}

// defaultHandlerUnimplemented 當任一種 API 收到未註冊的 method 時：寫 log 並回傳 gRPC Unimplemented。
func (s *Server) defaultHandlerUnimplemented(apiType, method string) error {
	log.Printf("[forward] %s 未註冊 %s %s", s.selfSvt, apiType, method)
	return status.Errorf(codes.Unimplemented, "no handler for %s", method)
}

// Request: request/response model.
func (s *Server) Request(ctx context.Context, req *gatepb.Pack) (*anypb.Any, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "nil pack")
	}
	method := req.GetMethod()
	if method == "" {
		return nil, status.Error(codes.InvalidArgument, "method is required")
	}

	// 從 metadata 提取 uid、traceId、callSource
	_, uidStr, traceId, callSource := extractMeta(ctx)
	var uid32 uint32
	if uidStr != "" {
		uid, err := strconv.ParseUint(uidStr, 10, 32)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid uid: %v", err)
		}
		uid32 = uint32(uid)
	} else if callSource != "service" {
		return nil, status.Error(codes.InvalidArgument, "uid missing in metadata")
	}
	// callSource == "service" 時 uid 可為 0（無對應用戶）

	if traceId != "" {
		ctx = svcerr.WithTraceID(ctx, traceId)
	}

	handler, ok := Get("request", method)
	if !ok {
		return nil, s.defaultHandlerUnimplemented("request", method)
	}
	info, err := handler(ctx, uid32, req.Info)
	if err != nil {
		return nil, svcerr.ToGRPCError(err)
	}
	return info, nil
}

// Notify: fire-and-forget with Pack payload.
func (s *Server) Notify(ctx context.Context, req *gatepb.Pack) (*emptypb.Empty, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "nil pack")
	}
	method := req.GetMethod()
	if method == "" {
		return nil, status.Error(codes.InvalidArgument, "method is required")
	}

	// 從 metadata 提取 uid、traceId、callSource
	_, uidStr, traceId, callSource := extractMeta(ctx)
	var uid32 uint32
	if uidStr != "" {
		uid, err := strconv.ParseUint(uidStr, 10, 32)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid uid: %v", err)
		}
		uid32 = uint32(uid)
	} else if callSource != "service" {
		return nil, status.Error(codes.InvalidArgument, "uid missing in metadata")
	}

	if traceId != "" {
		ctx = svcerr.WithTraceID(ctx, traceId)
	}

	handler, ok := Get("notify", method)
	if !ok {
		return nil, s.defaultHandlerUnimplemented("notify", method)
	}
	if _, err := handler(ctx, uid32, req.Info); err != nil {
		return nil, svcerr.ToGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// Trigger: no input, no output.
func (s *Server) Trigger(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	method, uidStr, traceId, callSource := extractMeta(ctx)
	if method == "" {
		return nil, status.Error(codes.InvalidArgument, "method missing in metadata")
	}
	log.Printf("[forward] %s 收到 trigger %s (uid=%s)", s.selfSvt, method, uidStr)
	var uid32 uint32
	if uidStr != "" {
		uid, err := strconv.ParseUint(uidStr, 10, 32)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid uid: %v", err)
		}
		uid32 = uint32(uid)
	} else if callSource != "service" {
		return nil, status.Error(codes.InvalidArgument, "uid missing in metadata")
	}

	// 將 traceId 存入 context
	if traceId != "" {
		ctx = svcerr.WithTraceID(ctx, traceId)
	}

	handler, ok := Get("trigger", method)
	if !ok {
		return nil, s.defaultHandlerUnimplemented("trigger", method)
	}
	if _, err := handler(ctx, uid32, nil); err != nil {
		return nil, svcerr.ToGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// Fetch: no input, return Any.
func (s *Server) Fetch(ctx context.Context, _ *emptypb.Empty) (*anypb.Any, error) {
	method, uidStr, traceId, callSource := extractMeta(ctx)
	if method == "" {
		return nil, status.Error(codes.InvalidArgument, "method missing in metadata")
	}
	var uid32 uint32
	if uidStr != "" {
		uid, err := strconv.ParseUint(uidStr, 10, 32)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid uid: %v", err)
		}
		uid32 = uint32(uid)
	} else if callSource != "service" {
		return nil, status.Error(codes.InvalidArgument, "uid missing in metadata")
	}

	// 將 traceId 存入 context
	if traceId != "" {
		ctx = svcerr.WithTraceID(ctx, traceId)
	}

	handler, ok := Get("fetch", method)
	if !ok {
		return nil, s.defaultHandlerUnimplemented("fetch", method)
	}
	info, err := handler(ctx, uid32, nil)
	if err != nil {
		return nil, svcerr.ToGRPCError(err)
	}
	return info, nil
}

// EntryAndLeave is not implemented in forward.Server.
// Each service should implement it themselves.
func (s *Server) EntryAndLeave(ctx context.Context, req *gatepb.EntryAndLeaveInfo) (*gatepb.EntryAndLeaveInfo, error) {
	_ = ctx
	_ = req
	return nil, status.Error(codes.Unimplemented, "method EntryAndLeave not implemented")
}

// Metadata key for service-to-service calls; when set to "service", uid is optional.
const MetadataCallSource = "x-call-source"

// extractMeta pulls method/uid/traceId/callSource from incoming gRPC metadata.
// When callSource == "service", uid may be empty (handler will receive uid 0).
func extractMeta(ctx context.Context) (method string, uid string, traceId string, callSource string) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("method"); len(vals) > 0 {
			method = vals[0]
		}
		if vals := md.Get("uid"); len(vals) > 0 {
			uid = vals[0]
		}
		if vals := md.Get("trace_id"); len(vals) > 0 {
			traceId = vals[0]
		}
		if vals := md.Get(MetadataCallSource); len(vals) > 0 {
			callSource = vals[0]
		}
	}
	return
}
