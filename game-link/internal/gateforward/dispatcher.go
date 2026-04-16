package gateforward

import (
	"context"
	"fmt"
	"strings"

	svcerr "internal/errors"
	forward "internal/grpc/forward"

	gatepb "internal.proto/pb/gate"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// RequestDispatcher 處理 request 封包：可為轉發或未來本機處理，由實作決定。
// Gate 可替換為直連 game service 等其他實作。
type RequestDispatcher interface {
	DispatchRequest(traceId string, uid uint32, pack *gatepb.Pack, msg proto.Message) (*anypb.Any, error)
}

// DefaultDispatcher 預設為 ForwardDispatcher；Gate 或其他服務可於 init 時替換。
var DefaultDispatcher RequestDispatcher = &ForwardDispatcher{}

// ForwardDispatcher 僅依 uid→sid 轉發，不處理 auth/validate（由 Gate 分支處理）。
type ForwardDispatcher struct{}

// DispatchRequest 要求 uid>0 並轉發；uid==0 回錯。
func (f *ForwardDispatcher) DispatchRequest(traceId string, uid uint32, pack *gatepb.Pack, msg proto.Message) (*anypb.Any, error) {
	if uid == 0 {
		return nil, fmt.Errorf("尚未認證，請先呼叫 auth/validate")
	}
	return ForwardRequest(traceId, uid, pack)
}

// ForwardRegistryDispatcher 直連用：依 forward 註冊表分發，僅處理 pack.Svt == expectedSvt 的 pack。
// 與 RPC 接收端共用同一套 forward.Request/Notify 註冊。
type ForwardRegistryDispatcher struct {
	expectedSvt string
}

// NewForwardRegistryDispatcher 建立依 forward 註冊表分發的 RequestDispatcher。
func NewForwardRegistryDispatcher(expectedSvt string) *ForwardRegistryDispatcher {
	return &ForwardRegistryDispatcher{expectedSvt: strings.TrimSpace(expectedSvt)}
}

// DispatchRequest 檢查 pack.Svt == expectedSvt，再依 pack.Method 呼叫 forward.Get("request", method)。
func (f *ForwardRegistryDispatcher) DispatchRequest(traceId string, uid uint32, pack *gatepb.Pack, msg proto.Message) (*anypb.Any, error) {
	if uid == 0 {
		return nil, fmt.Errorf("尚未認證，請先呼叫 auth/validate")
	}
	svt := strings.TrimSpace(pack.GetSvt())
	if svt != f.expectedSvt {
		return nil, fmt.Errorf("pack svt %q 與本服務 %q 不符", svt, f.expectedSvt)
	}
	method := strings.TrimSpace(pack.GetMethod())
	if method == "" {
		return nil, fmt.Errorf("method is required")
	}
	h, ok := forward.Get("request", method)
	if !ok || h == nil {
		return nil, fmt.Errorf("no handler for request %s", method)
	}
	ctx := context.Background()
	if traceId != "" {
		ctx = svcerr.WithTraceID(ctx, traceId)
	}
	return h(ctx, uid, pack.GetInfo())
}

// NotifyDispatcher 處理 notify 封包：可為轉發或本機處理。
type NotifyDispatcher interface {
	DispatchNotify(traceId string, uid uint32, pack *gatepb.Pack) error
}

// DefaultNotifyDispatcher 預設為 nil，route 使用 ForwardNotify；直連服務可設為 ForwardNotifyRegistryDispatcher。
var DefaultNotifyDispatcher NotifyDispatcher

// ForwardNotifyRegistryDispatcher 直連用：依 forward 註冊表分發 notify，僅處理 pack.Svt == expectedSvt 的 pack。
type ForwardNotifyRegistryDispatcher struct {
	expectedSvt string
}

// NewForwardNotifyRegistryDispatcher 建立依 forward 註冊表分發的 NotifyDispatcher。
func NewForwardNotifyRegistryDispatcher(expectedSvt string) *ForwardNotifyRegistryDispatcher {
	return &ForwardNotifyRegistryDispatcher{expectedSvt: strings.TrimSpace(expectedSvt)}
}

// DispatchNotify 檢查 pack.Svt == expectedSvt，再依 pack.Method 呼叫 forward.Get("notify", method)。
func (f *ForwardNotifyRegistryDispatcher) DispatchNotify(traceId string, uid uint32, pack *gatepb.Pack) error {
	svt := strings.TrimSpace(pack.GetSvt())
	if svt != f.expectedSvt {
		return fmt.Errorf("pack svt %q 與本服務 %q 不符", svt, f.expectedSvt)
	}
	method := strings.TrimSpace(pack.GetMethod())
	if method == "" {
		return fmt.Errorf("method is required")
	}
	h, ok := forward.Get("notify", method)
	if !ok || h == nil {
		return fmt.Errorf("no handler for notify %s", method)
	}
	ctx := context.Background()
	if traceId != "" {
		ctx = svcerr.WithTraceID(ctx, traceId)
	}
	_, err := h(ctx, uid, pack.GetInfo())
	return err
}

// TriggerDispatcher 處理 trigger 封包：可為轉發或本機處理。
type TriggerDispatcher interface {
	DispatchTrigger(traceId string, uid uint32, pack *gatepb.Pack) error
}

// DefaultTriggerDispatcher 預設為 nil，route 使用 ForwardTrigger；直連服務可設為 ForwardTriggerRegistryDispatcher。
var DefaultTriggerDispatcher TriggerDispatcher

// ForwardTriggerRegistryDispatcher 直連用：依 forward 註冊表分發 trigger，僅處理 pack.Svt == expectedSvt 的 pack。
type ForwardTriggerRegistryDispatcher struct {
	expectedSvt string
}

// NewForwardTriggerRegistryDispatcher 建立依 forward 註冊表分發的 TriggerDispatcher。
func NewForwardTriggerRegistryDispatcher(expectedSvt string) *ForwardTriggerRegistryDispatcher {
	return &ForwardTriggerRegistryDispatcher{expectedSvt: strings.TrimSpace(expectedSvt)}
}

// DispatchTrigger 檢查 pack.Svt == expectedSvt，再依 pack.Method 呼叫 forward.Get("trigger", method)。
func (f *ForwardTriggerRegistryDispatcher) DispatchTrigger(traceId string, uid uint32, pack *gatepb.Pack) error {
	svt := strings.TrimSpace(pack.GetSvt())
	if svt != f.expectedSvt {
		return fmt.Errorf("pack svt %q 與本服務 %q 不符", svt, f.expectedSvt)
	}
	method := strings.TrimSpace(pack.GetMethod())
	if method == "" {
		return fmt.Errorf("method is required")
	}
	h, ok := forward.Get("trigger", method)
	if !ok || h == nil {
		return fmt.Errorf("no handler for trigger %s", method)
	}
	ctx := context.Background()
	if traceId != "" {
		ctx = svcerr.WithTraceID(ctx, traceId)
	}
	_, err := h(ctx, uid, pack.GetInfo())
	return err
}
