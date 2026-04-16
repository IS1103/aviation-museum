package forward

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
)

// HandlerFunc defines a gRPC forward handler signature.
// funcName(ctx, uid, infoAny) -> respAny
type HandlerFunc func(ctx context.Context, uid uint32, info *anypb.Any) (*anypb.Any, error)

var registry = make(map[string]map[string]HandlerFunc) // method -> funcName -> handler

func register(method, funcName string, h HandlerFunc) {
	if _, ok := registry[method]; !ok {
		registry[method] = make(map[string]HandlerFunc)
	}
	registry[method][funcName] = h
}

// Get handler by method + funcName.
func Get(method, funcName string) (HandlerFunc, bool) {
	if m, ok := registry[method]; ok {
		h, ok2 := m[funcName]
		return h, ok2
	}
	return nil, false
}

// RegisterTrigger registers a trigger handler (no payload, no response).
// 注意：此函數被 generic.go 的 Trigger() 內部使用
func RegisterTrigger(funcName string, h func(ctx context.Context, uid uint32) error) {
	register("trigger", funcName, func(ctx context.Context, uid uint32, info *anypb.Any) (*anypb.Any, error) {
		if err := h(ctx, uid); err != nil {
			return nil, err
		}
		return nil, nil
	})
}
