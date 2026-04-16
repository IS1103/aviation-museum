package ws

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Request 註冊需要回應的 handler（自動驗證 UID + 自動綁定請求）
// 簡化前：
//
//	func LeaveHandler(ctx ws.WSContext) *anypb.Any {
//	    req := &pbgame.JoinLeave_Req{}
//	    uid := ctx.GetUID()
//	    if uid == 0 {
//	        ctx.Throw("請先進行認證")
//	        return nil
//	    }
//	    if err := ctx.ShouldBind(req); err != nil {
//	        ctx.Throw(fmt.Sprintf("解析請求失敗: %v", err))
//	        return nil
//	    }
//	    // 業務邏輯...
//	}
//
// 簡化後：
//
//	ws.Request("leave", func(ctx ws.WSContext, uid uint32, req *pbgame.JoinLeave_Req) (*pbgame.SomeResp, error) {
//	    // 直接寫業務邏輯，uid 已驗證，req 已綁定
//	})
func Request[Req, Resp proto.Message](route string, h func(ctx WSContext, uid uint32, req Req) (Resp, error)) {
	RegisterRequestHandler(route, nil, func(ctx WSContext) *anypb.Any {
		// 自動驗證 UID
		uid := ctx.GetUID()
		if uid == 0 {
			ctx.Throw("請先進行認證")
			return nil
		}

		// 自動綁定請求
		var req Req
		req = req.ProtoReflect().New().Interface().(Req)
		if err := ctx.ShouldBind(req); err != nil {
			ctx.Throw(fmt.Sprintf("解析請求失敗: %v", err))
			return nil
		}

		// 執行業務邏輯
		resp, err := h(ctx, uid, req)
		if err != nil {
			ctx.Throw(err.Error())
			return nil
		}

		// 包裝回應
		if proto.Message(resp) == nil || resp.ProtoReflect() == nil {
			return nil
		}
		anyResp, err := anypb.New(resp)
		if err != nil {
			ctx.Throw(fmt.Sprintf("包裝回應失敗: %v", err))
			return nil
		}
		return anyResp
	})
}

// Notify 註冊不需要回應的 handler（自動驗證 UID + 自動綁定請求）
// 簡化前：
//
//	func EntryHandler(ctx ws.WSContext) {
//	    req := &pbgame.JoinLeave_Req{}
//	    uid := ctx.GetUID()
//	    if uid == 0 {
//	        ctx.Throw("請先進行認證")
//	        return
//	    }
//	    if err := ctx.ShouldBind(req); err != nil {
//	        ctx.Throw(fmt.Sprintf("解析請求失敗: %v", err))
//	        return
//	    }
//	    // 業務邏輯...
//	}
//
// 簡化後：
//
//	ws.Notify("entry", func(ctx ws.WSContext, uid uint32, req *pbgame.JoinLeave_Req) error {
//	    // 直接寫業務邏輯，uid 已驗證，req 已綁定
//	})
func Notify[Req proto.Message](route string, h func(ctx WSContext, uid uint32, req Req) error) {
	RegisterNotifyHandler(route, nil, func(ctx WSContext) {
		// 自動驗證 UID
		uid := ctx.GetUID()
		if uid == 0 {
			ctx.Throw("請先進行認證")
			return
		}

		// 自動綁定請求
		var req Req
		req = req.ProtoReflect().New().Interface().(Req)
		if err := ctx.ShouldBind(req); err != nil {
			ctx.Throw(fmt.Sprintf("解析請求失敗: %v", err))
			return
		}

		// 執行業務邏輯
		if err := h(ctx, uid, req); err != nil {
			ctx.Throw(err.Error())
		}
	})
}

// Fetch 註冊無請求參數、需要回應的 handler（自動驗證 UID）
// 適用於取得資料的 API（如 getGameData、getProfile）
func Fetch[Resp proto.Message](route string, h func(ctx WSContext, uid uint32) (Resp, error)) {
	RegisterFetchHandler(route, nil, func(ctx WSContext) *anypb.Any {
		// 自動驗證 UID
		uid := ctx.GetUID()
		if uid == 0 {
			ctx.Throw("請先進行認證")
			return nil
		}

		// 執行業務邏輯
		resp, err := h(ctx, uid)
		if err != nil {
			ctx.Throw(err.Error())
			return nil
		}

		// 包裝回應
		if proto.Message(resp) == nil || resp.ProtoReflect() == nil {
			return nil
		}
		anyResp, err := anypb.New(resp)
		if err != nil {
			ctx.Throw(fmt.Sprintf("包裝回應失敗: %v", err))
			return nil
		}
		return anyResp
	})
}

// Trigger 註冊無請求參數、不需要回應的 handler（自動驗證 UID）
// 適用於觸發動作的 API（如 ready、startGame）
func Trigger(route string, h func(ctx WSContext, uid uint32) error) {
	RegisterTriggerHandler(route, nil, func(ctx WSContext) {
		// 自動驗證 UID
		uid := ctx.GetUID()
		if uid == 0 {
			ctx.Throw("請先進行認證")
			return
		}

		// 執行業務邏輯
		if err := h(ctx, uid); err != nil {
			ctx.Throw(err.Error())
		}
	})
}

// RequestNoAuth 註冊不需要驗證 UID 的 request handler（如 auth 本身）
// 僅自動綁定請求，不驗證 UID
func RequestNoAuth[Req, Resp proto.Message](route string, h func(ctx WSContext, req Req) (Resp, error)) {
	RegisterRequestHandler(route, nil, func(ctx WSContext) *anypb.Any {
		// 自動綁定請求（不驗證 UID）
		var req Req
		req = req.ProtoReflect().New().Interface().(Req)
		if err := ctx.ShouldBind(req); err != nil {
			ctx.Throw(fmt.Sprintf("解析請求失敗: %v", err))
			return nil
		}

		// 執行業務邏輯
		resp, err := h(ctx, req)
		if err != nil {
			ctx.Throw(err.Error())
			return nil
		}

		// 包裝回應
		if proto.Message(resp) == nil || resp.ProtoReflect() == nil {
			return nil
		}
		anyResp, err := anypb.New(resp)
		if err != nil {
			ctx.Throw(fmt.Sprintf("包裝回應失敗: %v", err))
			return nil
		}
		return anyResp
	})
}
