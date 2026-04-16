package forward

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Request 註冊 request handler（有 request、有 response）
// 用法：forward.Request("bet", Bet)
func Request[Req, Resp proto.Message](method string, h func(ctx context.Context, uid uint32, req Req) (Resp, error)) {
	register("request", method, func(ctx context.Context, uid uint32, info *anypb.Any) (*anypb.Any, error) {
		var req Req
		req = req.ProtoReflect().New().Interface().(Req)
		if info != nil {
			if err := info.UnmarshalTo(req); err != nil {
				return nil, err
			}
		}
		resp, err := h(ctx, uid, req)
		if err != nil {
			return nil, err
		}
		return anypb.New(resp)
	})
}

// Notify 註冊 notify handler（有 request、無 response）
// 用法：forward.Notify("chat", Chat)
func Notify[Req proto.Message](method string, h func(ctx context.Context, uid uint32, req Req) error) {
	register("notify", method, func(ctx context.Context, uid uint32, info *anypb.Any) (*anypb.Any, error) {
		var req Req
		req = req.ProtoReflect().New().Interface().(Req)
		if info != nil {
			if err := info.UnmarshalTo(req); err != nil {
				return nil, err
			}
		}
		if err := h(ctx, uid, req); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

// Fetch 註冊 fetch handler（無 request、有 response）
// 用法：forward.Fetch("getGameData", GetGameData)
func Fetch[Resp proto.Message](method string, h func(ctx context.Context, uid uint32) (Resp, error)) {
	register("fetch", method, func(ctx context.Context, uid uint32, info *anypb.Any) (*anypb.Any, error) {
		resp, err := h(ctx, uid)
		if err != nil {
			return nil, err
		}
		// 檢查 resp 是否為 nil（透過 proto.Message interface）
		if proto.Message(resp) == nil || resp.ProtoReflect() == nil {
			return nil, nil
		}
		return anypb.New(resp)
	})
}

// Trigger 註冊 trigger handler（無 request、無 response）
// 這個已經存在 RegisterTrigger，這裡提供簡短別名
// 用法：forward.Trigger("heartbeat", Heartbeat)
func Trigger(method string, h func(ctx context.Context, uid uint32) error) {
	RegisterTrigger(method, h)
}
