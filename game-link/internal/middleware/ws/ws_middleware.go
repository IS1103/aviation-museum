package ws

import (
	"fmt"
	"strings"
	"time"

	"internal/logger"
	"internal/webcore/context"
	wscore "internal/webcore/ws"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
)

// isGateRoute 判斷是否為 gate 服務的路由（需要顯示完整請求內容）
func isGateRoute(route string) bool {
	return strings.HasPrefix(route, "gate/")
}

// LoggingMiddleware 記錄 WS 收發與錯誤。
func LoggingMiddleware(ctx wscore.WSContext, next wscore.ResponseHandlerFunc) *anypb.Any {
	route := ctx.GetRoute()
	packType := ctx.GetUint32(context.PackType)
	callMethod := wscore.GetCallMethodName(packType)

	// 取得 traceId 與 uid（供日誌前綴）
	traceId := ""
	if t, ok := ctx.GetExtra("trace_id"); ok {
		traceId = t.(string)
	}
	logPrefix := logger.GateLogPrefix(ctx.GetUID(), traceId)

	// 記錄開始時間
	startTime := time.Now()
	ctx.SetExtra("_req_start_time", startTime)

	// 取得請求資訊
	reqInfo := ""
	if rawPack, ok := ctx.GetExtra("raw_pack"); ok {
		if pack, ok := rawPack.(interface{ GetInfo() *anypb.Any }); ok {
			if info := pack.GetInfo(); info != nil && info.TypeUrl != "" {
				if isGateRoute(route) {
					// gate 服務的路由：顯示完整的請求內容
					unmarshaled, err := context.UnmarshalAnyDynamic(info)
					if err == nil && unmarshaled != nil {
						marshaler := protojson.MarshalOptions{
							UseProtoNames:   true,
							EmitUnpopulated: false,
						}
						if jsonBytes, jsonErr := marshaler.Marshal(unmarshaled); jsonErr == nil {
							reqInfo = fmt.Sprintf(": %s", string(jsonBytes))
						} else {
							reqInfo = fmt.Sprintf(": %s", info.TypeUrl)
						}
					} else {
						reqInfo = fmt.Sprintf(": %s", info.TypeUrl)
					}
				} else {
					// 其他服務：只顯示 type URL
					reqInfo = fmt.Sprintf(": %s", info.TypeUrl)
				}
			}
		}
	}

	// 組合路由格式：callMethod/route（如 request/gate/entry）
	fullRoute := fmt.Sprintf("%s/%s", callMethod, route)

	// 記錄請求（使用 Gate 專用 logger，帶上 uid | traceID）；略過 gate/ping 以免干擾 debug
	if route != "gate/ping" {
		logger.GateInfo(fmt.Sprintf("%s[▶][%s]%s", logPrefix, fullRoute, reqInfo))
	}

	result := next(ctx)

	// 記錄錯誤（使用 Gate 專用 logger，帶上 uid | traceID）
	if err, ok := ctx.Get(context.Error); ok && err.(string) != "" {
		logger.GateError(fmt.Sprintf("%s[◀][%s] Error: %s", logPrefix, fullRoute, err.(string)))
	}

	return result
}
