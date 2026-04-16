package errors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	gatepb "internal.proto/pb/gate"
)

// traceIdKey 用於從 context 中提取 traceId
type traceIdKeyType struct{}

var traceIdKey = traceIdKeyType{}

// GetTraceID 從 context 中獲取 traceId
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value(traceIdKey); v != nil {
		return v.(string)
	}
	return ""
}

// WithTraceID 將 traceId 存入 context
func WithTraceID(ctx context.Context, traceId string) context.Context {
	return context.WithValue(ctx, traceIdKey, traceId)
}

// E 創建帶 traceId 和 stack trace 的錯誤
func E(ctx context.Context, msg string) error {
	traceId := GetTraceID(ctx)
	if traceId != "" {
		return errors.New(fmt.Sprintf("[%s] %s", traceId, msg))
	}
	return errors.New(msg)
}

// stackTracer 用於提取 stack trace
type stackTracer interface {
	StackTrace() errors.StackTrace
}

// ToGRPCError 將 error 轉換為帶 Details 的 gRPC error
func ToGRPCError(err error) error {
	if err == nil {
		return nil
	}

	detail := &gatepb.ErrorDetail{
		Msg: err.Error(),
	}

	// 提取 stack trace
	if st, ok := err.(stackTracer); ok {
		frames := st.StackTrace()
		if len(frames) > 0 {
			// 取得第一個 frame 作為位置
			detail.Loc = fmt.Sprintf("%s", frames[0])
			// 完整 stack
			detail.Stack = formatStack(frames)
		}
	}

	st, _ := status.New(codes.Unknown, err.Error()).WithDetails(detail)
	return st.Err()
}

// FromGRPCError 從 gRPC error 提取 ErrorDetail
// 回傳: msg (使用者訊息), detail JSON 字串
func FromGRPCError(err error) (msg string, detailJSON string) {
	if err == nil {
		return "", ""
	}

	st, ok := status.FromError(err)
	if !ok {
		return err.Error(), ""
	}

	msg = st.Message()

	for _, d := range st.Details() {
		if ed, ok := d.(*gatepb.ErrorDetail); ok {
			// 轉成 JSON
			jsonBytes, _ := json.Marshal(map[string]string{
				"loc":   ed.Loc,
				"stack": ed.Stack,
			})
			return msg, string(jsonBytes)
		}
	}

	return msg, ""
}

// formatStack 格式化 stack trace
func formatStack(frames errors.StackTrace) string {
	var parts []string
	limit := 5 // 最多顯示 5 層
	for i, frame := range frames {
		if i >= limit {
			break
		}
		parts = append(parts, fmt.Sprintf("%s", frame))
	}
	return strings.Join(parts, "→")
}
