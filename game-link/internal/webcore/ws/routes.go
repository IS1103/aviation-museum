package ws

import (
	"strings"

	gatepb "internal.proto/pb/gate"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// HandlerInfo stores typed handlers and ctor.
type HandlerInfo struct {
	Ctor          func() proto.Message
	RequestHandle func(ctx WSContext, uid uint32, pack *gatepb.Pack, msg proto.Message) (*anypb.Any, error)
	NotifyHandle  func(ctx WSContext, uid uint32, pack *gatepb.Pack, msg proto.Message) error
	HasResponse   bool // true for request/fetch
}

// PackType 與 gate.Pack 一致：0=request, 1=response, 2=notify, 3=trigger, 4=fetch
const (
	PackTypeRequest = 0
	PackTypeNotify  = 2
	PackTypeTrigger = 3
	PackTypeFetch   = 4
)

// HandlerRegistry 精確匹配：svt（給哪個 service）+ packType（用什麼方式處理）+ method（目標 service 收到後要執行的 method）。
// fallbackByPackType 僅供 gate 轉發用：精確未命中時依 packType 選一個 handler。
type HandlerRegistry struct {
	// exactHandlers[svt][packType][method]：哪個服務、哪種封包類型、哪個方法
	exactHandlers      map[string]map[uint32]map[string]*HandlerInfo
	fallbackByPackType map[uint32]*HandlerInfo
}

var registry = &HandlerRegistry{
	exactHandlers:      make(map[string]map[uint32]map[string]*HandlerInfo),
	fallbackByPackType: make(map[uint32]*HandlerInfo),
}

// wsHandlerRegistry 字串路由索引，供 GetAllRoutes / GetRouteHandler 使用
var wsHandlerRegistry = make(map[string]*HandlerInfo)

// RegisterRequestHandler 註冊 request handler，簽名固定為 (route, ctor, handler)。
// ctor 可為 nil（不解析 payload）；handler 為 (ctx, uid, pack, msg) -> (*anypb.Any, error)。
func RegisterRequestHandler(route string, ctor func() proto.Message, handler interface{}) {
	handlerInfo := &HandlerInfo{
		Ctor:        ctor,
		HasResponse: true,
	}

	switch h := handler.(type) {
	case func(ctx WSContext, uid uint32, pack *gatepb.Pack, msg proto.Message) (*anypb.Any, error):
		handlerInfo.RequestHandle = h
	case func(ctx WSContext) *anypb.Any:
		handlerInfo.RequestHandle = func(ctx WSContext, uid uint32, pack *gatepb.Pack, msg proto.Message) (*anypb.Any, error) {
			return h(ctx), nil
		}
	default:
		return
	}

	wsHandlerRegistry[route] = handlerInfo
	registerRoute(route, handlerInfo, PackTypeRequest)
}

// RegisterNotifyHandler 註冊 notify handler，簽名固定為 (route, ctor, handler)。
func RegisterNotifyHandler(route string, ctor func() proto.Message, handler interface{}) {
	handlerInfo := &HandlerInfo{
		Ctor:        ctor,
		HasResponse: false,
	}

	switch h := handler.(type) {
	case func(ctx WSContext, uid uint32, pack *gatepb.Pack, msg proto.Message) error:
		handlerInfo.NotifyHandle = h
	case func(ctx WSContext):
		handlerInfo.NotifyHandle = func(ctx WSContext, uid uint32, pack *gatepb.Pack, msg proto.Message) error {
			h(ctx)
			return nil
		}
	default:
		return
	}

	wsHandlerRegistry[route] = handlerInfo
	registerRoute(route, handlerInfo, PackTypeNotify)
}

// RegisterFetchHandler 註冊 fetch handler，簽名固定為 (route, ctor, handler)。
func RegisterFetchHandler(route string, ctor func() proto.Message, handler interface{}) {
	handlerInfo := &HandlerInfo{
		Ctor:        ctor,
		HasResponse: true,
	}

	switch h := handler.(type) {
	case func(ctx WSContext, uid uint32, pack *gatepb.Pack, msg proto.Message) (*anypb.Any, error):
		handlerInfo.RequestHandle = h
	case func(ctx WSContext) *anypb.Any:
		handlerInfo.RequestHandle = func(ctx WSContext, uid uint32, pack *gatepb.Pack, msg proto.Message) (*anypb.Any, error) {
			return h(ctx), nil
		}
	default:
		return
	}

	wsHandlerRegistry[route] = handlerInfo
	registerRoute(route, handlerInfo, PackTypeFetch)
}

// RegisterTriggerHandler 註冊 trigger handler，簽名固定為 (route, ctor, handler)。
func RegisterTriggerHandler(route string, ctor func() proto.Message, handler interface{}) {
	handlerInfo := &HandlerInfo{
		Ctor:        ctor,
		HasResponse: false,
	}

	switch h := handler.(type) {
	case func(ctx WSContext, uid uint32, pack *gatepb.Pack, msg proto.Message) error:
		handlerInfo.NotifyHandle = h
	case func(ctx WSContext):
		handlerInfo.NotifyHandle = func(ctx WSContext, uid uint32, pack *gatepb.Pack, msg proto.Message) error {
			h(ctx)
			return nil
		}
	default:
		return
	}

	wsHandlerRegistry[route] = handlerInfo
	registerRoute(route, handlerInfo, PackTypeTrigger)
}

// routeToPackType 僅供 gate 註冊 "request"/"notify"/"trigger"/"fetch" 時寫入 fallbackByPackType
var routeToPackType = map[string]uint32{
	"request": PackTypeRequest,
	"notify":  PackTypeNotify,
	"trigger": PackTypeTrigger,
	"fetch":   PackTypeFetch,
}

// registerRoute 精確匹配：svt + packType + method。若 route 為 "request"/"notify"/"trigger"/"fetch" 則寫入 fallbackByPackType。
func registerRoute(route string, handlerInfo *HandlerInfo, packType uint32) {
	parts := strings.Split(route, "/")
	if len(parts) == 2 {
		svt, method := parts[0], parts[1]
		if registry.exactHandlers[svt] == nil {
			registry.exactHandlers[svt] = make(map[uint32]map[string]*HandlerInfo)
		}
		if registry.exactHandlers[svt][packType] == nil {
			registry.exactHandlers[svt][packType] = make(map[string]*HandlerInfo)
		}
		registry.exactHandlers[svt][packType][method] = handlerInfo
	} else if len(parts) == 1 {
		if pt, ok := routeToPackType[route]; ok {
			registry.fallbackByPackType[pt] = handlerInfo
		}
	}
}

// GetHandler 精確匹配：svt（給哪個 service）+ packType（用什麼方式處理）+ method（目標 service 收到後要執行的 method）。未命中時回傳 fallback（供 gate 轉發）。
func GetHandler(svt, method string, packType uint32) (*HandlerInfo, bool) {
	if svt != "" && method != "" {
		if byPackType, ok := registry.exactHandlers[svt]; ok {
			if byMethod, ok := byPackType[packType]; ok {
				if h, ok := byMethod[method]; ok {
					return h, true
				}
			}
		}
	}
	if h, ok := registry.fallbackByPackType[packType]; ok {
		return h, true
	}
	return nil, false
}

// GetRouteHandler 依字串路由取得 handler
func GetRouteHandler(route string) (*HandlerInfo, bool) {
	h, ok := wsHandlerRegistry[route]
	return h, ok
}

func GetAllRoutes() []string {
	routes := make([]string, 0, len(wsHandlerRegistry))
	for route := range wsHandlerRegistry {
		routes = append(routes, route)
	}
	return routes
}
