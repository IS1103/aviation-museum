package gateforward

import (
	"fmt"

	webcontext "internal/webcore/context"
	"internal/webcore/ws"

	gatepb "internal.proto/pb/gate"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// AuthValidateFunc 由各服務（如 Gate）實作，處理 auth/validate；直連 game service 可有不同邏輯。
// req/resp 為 client 接發的 gate.ValidateReq / gate.ValidateResp。
type AuthValidateFunc func(ctx ws.WSContext, req *gatepb.ValidateReq) (*gatepb.ValidateResp, error)

var authValidateHandler AuthValidateFunc

// RegisterGateRoutes 註冊 request/notify/trigger/fetch，並注入 auth/validate 的處理函式。
// 由 Gate（或未來直連 game service）在 init 時呼叫，傳入各自的 HandleAuthValidate。
func RegisterGateRoutes(authHandler AuthValidateFunc) {
	authValidateHandler = authHandler
	registerRoutes()
}

func registerRoutes() {
	ws.RegisterRequestHandler("request", nil, routeHandler)
	ws.RegisterNotifyHandler("notify", nil, func(ctx ws.WSContext, uid uint32, pack *gatepb.Pack, msg proto.Message) error {
		_, err := routeHandler(ctx, uid, pack, msg)
		return err
	})
	ws.RegisterTriggerHandler("trigger", nil, func(ctx ws.WSContext, uid uint32, pack *gatepb.Pack, msg proto.Message) error {
		_, err := routeHandler(ctx, uid, pack, msg)
		return err
	})
	ws.RegisterFetchHandler("fetch", nil, routeHandler)
}

// routeHandler 只做分支：request 時 auth/validate→注入的 handler，其餘→Dispatcher；notify/trigger/fetch→轉發。
func routeHandler(wsCtx ws.WSContext, uid uint32, pack *gatepb.Pack, msg proto.Message) (*anypb.Any, error) {
	packType := pack.GetPackType()
	traceId := ""
	if tid, ok := wsCtx.GetExtra("trace_id"); ok {
		traceId = tid.(string)
	}

	if packType == 0 {
		if pack.GetSvt() == "auth" && pack.GetMethod() == "validate" {
			if authValidateHandler != nil {
				req := &gatepb.ValidateReq{}
				if pack.GetInfo() != nil {
					if err := pack.GetInfo().UnmarshalTo(req); err != nil {
						return nil, fmt.Errorf("invalid auth/validate payload: %w", err)
					}
				}
				resp, err := authValidateHandler(wsCtx, req)
				if err != nil {
					return nil, err
				}
				if resp == nil {
					return nil, nil
				}
				return anypb.New(resp)
			}
			return nil, fmt.Errorf("auth/validate handler not registered")
		}
		if pack.GetSvt() == "gate" && pack.GetMethod() == "ping" {
			return anypb.New(&gatepb.PingResp{})
		}
		return DefaultDispatcher.DispatchRequest(traceId, uid, pack, msg)
	}

	if uid == 0 {
		return handleErrorByPackType(wsCtx, "尚未認證，請先呼叫 auth/validate")
	}

	svt := pack.GetSvt()
	method := pack.GetMethod()
	if svt == "" {
		return handleErrorByPackType(wsCtx, "svt is required")
	}
	if method == "" {
		return handleErrorByPackType(wsCtx, "method is required")
	}

	switch packType {
	case 2:
		if DefaultNotifyDispatcher != nil {
			if err := DefaultNotifyDispatcher.DispatchNotify(traceId, uid, pack); err != nil {
				return handleErrorByPackType(wsCtx, fmt.Sprintf("local notify failed: %v", err))
			}
		} else {
			if err := ForwardNotify(traceId, uid, pack); err != nil {
				return handleErrorByPackType(wsCtx, fmt.Sprintf("forward notify failed: %v", err))
			}
		}
		return nil, nil
	case 3:
		if DefaultTriggerDispatcher != nil {
			if err := DefaultTriggerDispatcher.DispatchTrigger(traceId, uid, pack); err != nil {
				return handleErrorByPackType(wsCtx, fmt.Sprintf("local trigger failed: %v", err))
			}
		} else {
			if err := ForwardTrigger(traceId, uid, pack); err != nil {
				return handleErrorByPackType(wsCtx, fmt.Sprintf("forward trigger failed: %v", err))
			}
		}
		return nil, nil
	case 4:
		resp, err := ForwardFetch(traceId, uid, pack)
		if err != nil {
			return nil, fmt.Errorf("forward fetch failed: %w", err)
		}
		return resp, nil
	default:
		return handleErrorByPackType(wsCtx, "unsupported packType")
	}
}

func handleErrorByPackType(ctx ws.WSContext, errMsg string) (*anypb.Any, error) {
	packType := ctx.GetUint32(webcontext.PackType)
	if packType == 2 || packType == 3 {
		ctx.Throw(errMsg)
		return nil, nil
	}
	return nil, fmt.Errorf("%s", errMsg)
}
