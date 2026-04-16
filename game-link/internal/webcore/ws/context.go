package ws

import (
	stdcontext "context"
	"fmt"
	"sync"
	"time"

	svcerr "internal/errors"
	webcontext "internal/webcore/context"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/types/known/anypb"
)

// WSContext WebSocket 專屬上下文接口（擴展基礎 Context），並嵌入 context.Context 以便直接傳入 gateforward 等 RPC 呼叫。
type WSContext interface {
	webcontext.Context   // 繼承基礎接口
	stdcontext.Context   // 嵌入，使 HandleAuthValidate 等可將 ctx 直接傳入 CallServiceRequestWithInfo
	SetUID(uid uint32)
	GetUID() uint32
}

// ResponseHandlerFunc 定義 WS Request/Response handler 函數簽名（返回 *anypb.Any）
type ResponseHandlerFunc func(ctx WSContext) *anypb.Any

// NotifyHandlerFunc 定義 WS Notify handler 函數簽名（無返回值）
type NotifyHandlerFunc func(ctx WSContext)

// MiddlewareFunc 定義 WS 中間件函數簽名（需要返回 *anypb.Any）
type MiddlewareFunc func(ctx WSContext, next ResponseHandlerFunc) *anypb.Any

// WsContext 實現 WSContext 接口，並實作 context.Context 以便直接傳入 gateforward 等 RPC 呼叫。
type WsContext struct {
	*webcontext.BaseContext // 指針嵌入基礎上下文，繼承共用方法
	Conn                    *websocket.Conn
	uid                     uint32 // WebSocket 專用的用戶 ID（uint32 類型）
	callCtx                 stdcontext.Context // 供 RPC 使用，含 trace_id；Value 委派給此
}

// wsContextPool WSContext 物件池
var wsContextPool = sync.Pool{
	New: func() interface{} {
		return &WsContext{
			BaseContext: webcontext.NewBaseContextPtr(),
			Conn:        nil,
			callCtx:     stdcontext.Background(),
		}
	},
}

// GetWsContext 從池中獲取 WsContext
func GetWsContext(conn *websocket.Conn) *WsContext {
	ctx := wsContextPool.Get().(*WsContext)
	ctx.BaseContext.Clear()
	ctx.Conn = conn
	ctx.uid = 0 // 清空 uid
	ctx.callCtx = stdcontext.Background() // 重置，避免池化物件殘留舊 trace_id
	return ctx
}

// PutWsContext 將 WsContext 放回池中
func PutWsContext(ctx *WsContext) {
	if ctx != nil {
		ctx.Conn = nil
		wsContextPool.Put(ctx)
	}
}

// SetExtra 設置額外數據（非枚舉欄位）。當 key 為 "trace_id" 時，同時更新 callCtx 供 context.Context 委派。
func (c *WsContext) SetExtra(key string, value interface{}) {
	c.BaseContext.SetExtra(key, value)
	if key == "trace_id" && value != nil {
		if tidStr, ok := value.(string); ok && tidStr != "" {
			c.callCtx = svcerr.WithTraceID(stdcontext.Background(), tidStr)
		}
	}
}

// Throw 返回錯誤響應（統一使用 Error，packType 由下游決定回應/推播）
func (c *WsContext) Throw(errMsg string) {
	c.Set(webcontext.Error, errMsg)

	// 調試信息
	if errMsg == "" {
		fmt.Printf("警告: ctx.Throw 被調用時傳入了空字符串\n")
	}
}

// SetUID 設置用戶 ID（uint32 類型）
func (c *WsContext) SetUID(uid uint32) {
	c.uid = uid
}

// GetUID 獲取用戶 ID（uint32 類型）
func (c *WsContext) GetUID() uint32 {
	return c.uid
}

// Deadline、Done、Err、Value 實作 context.Context，委派給 callCtx，使 *WsContext 可直接傳入 CallServiceRequestWithInfo 等。
func (c *WsContext) Deadline() (deadline time.Time, ok bool) { return c.callCtx.Deadline() }
func (c *WsContext) Done() <-chan struct{}                   { return c.callCtx.Done() }
func (c *WsContext) Err() error                              { return c.callCtx.Err() }
func (c *WsContext) Value(key interface{}) interface{}       { return c.callCtx.Value(key) }

// GetWSConn 從 Context 中獲取 WebSocket 連接
func GetWSConn(ctx webcontext.Context) *websocket.Conn {
	if wsCtx, ok := ctx.(*WsContext); ok {
		return wsCtx.Conn
	}
	return nil
}

// SendMessage 通過 WebSocket 發送 protobuf 消息
func (c *WsContext) SendMessage(ctx stdcontext.Context, data []byte) error {
	return c.Conn.Write(ctx, websocket.MessageBinary, data)
}
