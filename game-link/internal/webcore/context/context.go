package context

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
)

// ContextKey 定義上下文鍵的枚舉
type ContextKey int

const (
	// 常用欄位 - 使用枚舉提升效能
	ReqID ContextKey = iota
	Route
	PackType
	UserID
	UUID
	Method     // HTTP 方法（GET, POST 等）
	Path       // HTTP 路徑
	Svt        // 服務類型（baccarat, holdem, match 等）
	PackMethod // Pack 方法名（bet, chat, entry 等，與 HTTP Method 區分）
	ReqData    // 請求數據（proto.Message 或 *anypb.Any）
	Error      // 錯誤訊息（保持原值不變）

	// 不常用欄位 - 使用字串 key（保持靈活性）
	// 這些會存儲到 extraData 中
)

// Context 定義基礎上下文接口（WS 和 HTTP 共用）
type Context interface {
	// Set 設置上下文數據（枚舉版本 - 高效能）
	Set(key ContextKey, value interface{})

	// Get 獲取上下文數據（枚舉版本 - 高效能）
	Get(key ContextKey) (interface{}, bool)

	// MustGet 獲取上下文數據，如果不存在則 panic
	MustGet(key ContextKey) interface{}

	// GetString 獲取字符串類型的上下文數據
	GetString(key ContextKey) string

	// GetInt32 獲取 int32 類型的上下文數據
	GetInt32(key ContextKey) int32

	// GetUint32 獲取 uint32 類型的上下文數據
	GetUint32(key ContextKey) uint32

	// GetBool 獲取 bool 類型的上下文數據
	GetBool(key ContextKey) bool

	// SetExtra 設置額外數據（非枚舉欄位）
	SetExtra(key string, value interface{})

	// GetExtra 獲取額外數據（非枚舉欄位）
	GetExtra(key string) (interface{}, bool)

	// ShouldBind 綁定請求數據到 proto message
	ShouldBind(msg proto.Message) error

	// Throw 返回錯誤響應
	Throw(errMsg string)

	// GetReqID 獲取請求 ID
	GetReqID() int32

	// GetRoute 獲取路由
	GetRoute() string
}

// BaseContext 基礎上下文實現（混合設計：常用欄位使用具體類型）
type BaseContext struct {
	// 常用欄位 - 使用具體類型，提升效能和類型安全
	ReqID      int32       // 請求 ID
	Route      string      // 路由名稱
	PackType   uint32      // 包類型 (0=request, 1=response, 2=notify, 3=trigger, 4=fetch)
	UserID     uint32      // 用戶 ID
	UUID       string      // 用戶 UUID
	Method     string      // HTTP 方法（GET, POST 等）
	Path       string      // HTTP 路徑
	Svt        string      // 服務類型（baccarat, holdem, match 等）
	PackMethod string      // Pack 方法名（bet, chat, entry 等，與 HTTP Method 區分）
	ReqData    interface{} // 請求數據（proto.Message 或 *anypb.Any，允許為 nil）
	Error      string      // 錯誤訊息

	// 不常用欄位 - 使用 interface{}，保持靈活性
	extraData map[string]interface{}
}

// NewBaseContext 創建基礎上下文（返回值，用於值嵌入）
func NewBaseContext() BaseContext {
	return BaseContext{
		extraData: make(map[string]interface{}),
	}
}

// NewBaseContextPtr 創建基礎上下文指針（用於池化）
func NewBaseContextPtr() *BaseContext {
	return &BaseContext{
		extraData: make(map[string]interface{}),
	}
}

// Clear 清空上下文資料但保留 map 容量
func (b *BaseContext) Clear() {
	// 清空常用欄位
	b.ReqID = 0
	b.Route = ""
	b.PackType = 0
	b.UserID = 0
	b.UUID = ""
	b.Method = ""
	b.Path = ""
	b.Svt = ""
	b.PackMethod = ""
	b.ReqData = nil
	b.Error = ""

	// 清空額外資料但保留容量
	for k := range b.extraData {
		delete(b.extraData, k)
	}
}

// Set 設置上下文數據（枚舉版本 - 高效能）
func (b *BaseContext) Set(key ContextKey, value interface{}) {
	switch key {
	case ReqID:
		if v, ok := value.(int32); ok {
			b.ReqID = v
		} else if v, ok := value.(uint32); ok {
			b.ReqID = int32(v)
		}
	case Route:
		if v, ok := value.(string); ok {
			b.Route = v
		}
	case PackType:
		if v, ok := value.(uint32); ok {
			b.PackType = v
		} else if v, ok := value.(int32); ok {
			// 兼容 int32，轉換為 uint32
			if v >= 0 {
				b.PackType = uint32(v)
			}
		}
	case UserID:
		if v, ok := value.(uint32); ok {
			b.UserID = v
		} else if v, ok := value.(int32); ok {
			b.UserID = uint32(v)
		} else if v, ok := value.(string); ok {
			// 如果是 string 類型，存儲到 extraData
			if b.extraData == nil {
				b.extraData = make(map[string]interface{})
			}
			b.extraData["_uid_string"] = v
		}
	case UUID:
		if v, ok := value.(string); ok {
			b.UUID = v
		}
	case Method:
		if v, ok := value.(string); ok {
			b.Method = v
		}
	case Path:
		if v, ok := value.(string); ok {
			b.Path = v
		}
	case Svt:
		if v, ok := value.(string); ok {
			b.Svt = v
		}
	case PackMethod:
		if v, ok := value.(string); ok {
			b.PackMethod = v
		}
	case ReqData:
		// 允許存儲 proto.Message 或 *anypb.Any 或 nil
		b.ReqData = value
	case Error:
		if v, ok := value.(string); ok {
			b.Error = v
		}
	}
}

// Get 獲取上下文數據（枚舉版本 - 高效能）
func (b *BaseContext) Get(key ContextKey) (interface{}, bool) {
	switch key {
	case ReqID:
		return b.ReqID, true
	case Route:
		return b.Route, true
	case PackType:
		return b.PackType, true
	case UserID:
		// 優先檢查是否有 string 類型的 uid
		if b.extraData != nil {
			if uidStr, ok := b.extraData["_uid_string"]; ok {
				return uidStr, true
			}
		}
		return b.UserID, true
	case UUID:
		return b.UUID, true
	case Method:
		return b.Method, true
	case Path:
		return b.Path, true
	case Svt:
		return b.Svt, true
	case PackMethod:
		return b.PackMethod, true
	case ReqData:
		return b.ReqData, b.ReqData != nil
	case Error:
		return b.Error, true
	default:
		return nil, false
	}
}

// MustGet 獲取上下文數據，如果不存在則 panic
func (b *BaseContext) MustGet(key ContextKey) interface{} {
	val, ok := b.Get(key)
	if !ok {
		panic(fmt.Sprintf("key %d not found in context", key))
	}
	return val
}

// GetString 獲取字符串類型的上下文數據（枚舉版本 - 高效能）
func (b *BaseContext) GetString(key ContextKey) string {
	switch key {
	case Route:
		return b.Route
	case UUID:
		return b.UUID
	case Method:
		return b.Method
	case Path:
		return b.Path
	case Svt:
		return b.Svt
	case PackMethod:
		return b.PackMethod
	case Error:
		return b.Error
	default:
		return ""
	}
}

// GetInt32 獲取 int32 類型的上下文數據（枚舉版本 - 高效能）
func (b *BaseContext) GetInt32(key ContextKey) int32 {
	switch key {
	case ReqID:
		return b.ReqID
	default:
		return 0
	}
}

// GetUint32 獲取 uint32 類型的上下文數據（枚舉版本 - 高效能）
func (b *BaseContext) GetUint32(key ContextKey) uint32 {
	switch key {
	case PackType:
		return b.PackType
	case UserID:
		return b.UserID
	default:
		return 0
	}
}

// GetBool 獲取 bool 類型的上下文數據（枚舉版本 - 高效能）
func (b *BaseContext) GetBool(key ContextKey) bool {
	switch key {
	default:
		return false
	}
}

// GetReqID 獲取請求 ID（優化：直接返回具體欄位）
func (b *BaseContext) GetReqID() int32 {
	return b.ReqID
}

// GetRoute 獲取路由（優化：直接返回具體欄位）
func (b *BaseContext) GetRoute() string {
	return b.Route
}

// SetExtra 設置額外數據（非枚舉欄位）
func (b *BaseContext) SetExtra(key string, value interface{}) {
	b.extraData[key] = value
}

// GetExtra 獲取額外數據（非枚舉欄位）
func (b *BaseContext) GetExtra(key string) (interface{}, bool) {
	val, ok := b.extraData[key]
	return val, ok
}

// ShouldBind 綁定請求數據到 proto message
// 如果沒有 request info，返回 nil（不報錯），允許某些 API 不需要請求參數
func (b *BaseContext) ShouldBind(msg proto.Message) error {
	reqData, ok := b.Get(ReqData)
	if !ok || reqData == nil {
		// 沒有 request info，這是允許的（某些 API 不需要請求參數）
		// 返回 nil 表示成功，但不會填充 msg
		return nil
	}

	// 如果已經是對應的 proto.Message 類型，直接拷貝
	if srcMsg, ok := reqData.(proto.Message); ok {
		proto.Merge(msg, srcMsg)
		return nil
	}

	// 如果是 Any 類型，解析
	if anyMsg, ok := reqData.(*anypb.Any); ok {
		return anyMsg.UnmarshalTo(msg)
	}

	return fmt.Errorf("ReqData type not supported (expected proto.Message or *anypb.Any)")
}

// UnmarshalAnyDynamic 動態解析 Any 類型消息（共用函數）
func UnmarshalAnyDynamic(anymsg *anypb.Any) (proto.Message, error) {
	mt, err := protoregistry.GlobalTypes.FindMessageByURL(anymsg.TypeUrl)
	if err != nil {
		return nil, err
	}
	msg := mt.New().Interface()
	if err := anymsg.UnmarshalTo(msg); err != nil {
		return nil, err
	}
	return msg, nil
}
