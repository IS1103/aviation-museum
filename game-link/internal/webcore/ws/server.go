package ws

import (
	stdcontext "context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	svcerr "internal/errors"
	"internal/logger"
	"internal/middleware/common"
	connmgr "internal/webcore/conn"
	webcontext "internal/webcore/context"

	"github.com/coder/websocket"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	gatepb "internal.proto/pb/gate"
)

// Server WebSocket 服務端
type Server struct {
	Port         string
	Middleware   []MiddlewareFunc
	PingInterval time.Duration // Ping 間隔時間，0 表示禁用
	PingTimeout  time.Duration // Ping 超時時間

	// PreAccept 可選：在 websocket.Accept（upgrade）之前執行。
	// 用於在「還沒建立 WS 連線」前就拒絕（例如 token 驗證），避免 client 看到 onopen 後立刻被踢。
	// 返回：
	// - uid：若 >0，會在 Accept 成功後自動 ctx.SetUID(uid)
	// - extras：若非空，會在 Accept 成功後自動 ctx.SetExtra(key,val)
	// - error：會直接回 HTTP 401 並 return（不 upgrade）
	PreAccept func(r *http.Request) (uid uint32, extras map[string]any, err error)

	// OnConnect 可選：在 WebSocket Accept（upgrade）成功後、進入消息循環前執行。
	// 若返回 error，服務端會關閉連線並拒絕後續消息處理。
	// 注意：此 hook 應避免將 token 等敏感資訊寫入錯誤字串，避免落入日誌。
	OnConnect func(ctx *WsContext, r *http.Request) error
}

// buildErrorMessage 使用 strings.Builder 優化錯誤訊息構建
func buildErrorMessage(prefix string, err error) string {
	var builder strings.Builder
	builder.Grow(len(prefix) + len(err.Error()) + 2) // 預分配容量
	builder.WriteString(prefix)
	builder.WriteString(": ")
	builder.WriteString(err.Error())
	return builder.String()
}

// buildRouteErrorMessage 使用 strings.Builder 優化路由錯誤訊息構建
func buildRouteErrorMessage(route, message string) string {
	var builder strings.Builder
	builder.Grow(len(route) + len(message) + 2) // 預分配容量
	builder.WriteString(route)
	builder.WriteString(" ")
	builder.WriteString(message)
	return builder.String()
}

// buildServerAddress 使用 strings.Builder 優化服務器地址構建
func buildServerAddress(port string) string {
	var builder strings.Builder
	builder.Grow(len(port) + 1) // 預分配容量
	builder.WriteString(":")
	builder.WriteString(port)
	return builder.String()
}

// NewServer 創建 WebSocket 服務端
func NewServer(port string, mws ...MiddlewareFunc) *Server {
	return &Server{
		Port:         port,
		Middleware:   mws,
		PingInterval: 30 * time.Second, // 默認每 30 秒 ping 一次
		PingTimeout:  10 * time.Second, // 默認 10 秒超時
	}
}

// SetPingConfig 設置 Ping/Pong 配置
func (s *Server) SetPingConfig(interval, timeout time.Duration) {
	s.PingInterval = interval
	s.PingTimeout = timeout
}

// ApplyRoutes 註冊 /ws 路由，所有 WebSocket 消息走統一的 dispatcher
func ApplyRoutes(server *Server) {
	http.HandleFunc("/ws", server.handleWS())
}

// handleWS WebSocket 處理函數
func (s *Server) handleWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var preUID uint32
		var preExtras map[string]any
		if s.PreAccept != nil {
			uid, extras, err := s.PreAccept(r)
			if err != nil {
				logger.Warn("WebSocket pre-accept rejected",
					zap.String("remoteAddr", r.RemoteAddr),
					zap.String("userAgent", r.UserAgent()),
					zap.Error(err),
				)
				// 直接拒絕 upgrade，讓 client 在 connect 階段就失敗
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("unauthorized"))
				return
			}
			preUID = uid
			preExtras = extras
		}

		opts := &websocket.AcceptOptions{
			OriginPatterns: []string{"*"},
		}
		conn, err := websocket.Accept(w, r, opts)
		if err != nil {
			logger.Error("WebSocket accept error",
				zap.String("remoteAddr", r.RemoteAddr),
				zap.Error(err),
			)
			return
		}

		// 為這個連接從池中獲取上下文
		wsCtx := GetWsContext(conn)
		defer PutWsContext(wsCtx)

		// 套用 PreAccept 的結果（避免重複驗證）
		if preUID != 0 {
			wsCtx.SetUID(preUID)
		}
		if len(preExtras) > 0 {
			for k, v := range preExtras {
				wsCtx.SetExtra(k, v)
			}
		}

		// 可選：在進入消息循環前執行連線初始化/驗證
		if s.OnConnect != nil {
			if err := s.OnConnect(wsCtx, r); err != nil {
				logger.Warn("WebSocket connect rejected",
					zap.String("remoteAddr", r.RemoteAddr),
					zap.String("userAgent", r.UserAgent()),
					zap.Error(err),
				)
				_ = conn.Close(websocket.StatusPolicyViolation, "unauthorized")
				return
			}
		}

		// 在連接關閉時清理用戶連接
		defer func() {
			// 如果用戶已認證，從連接管理器中移除
			if uid := wsCtx.GetUID(); uid != 0 {
				connmgr.GetConnectionManager().Unregister(uid)
			}
			conn.Close(websocket.StatusInternalError, "server closed")
		}()

		// 創建一個可取消的 context
		readerCtx, cancelReader := stdcontext.WithCancel(stdcontext.Background())
		defer cancelReader()

		// 啟動 Ping/Pong 監控 goroutine（如果啟用）
		if s.PingInterval > 0 {
			go s.pingLoop(readerCtx, conn, r.RemoteAddr)
		}

		// 消息循環
		for {
			typ, msg, err := conn.Reader(readerCtx)
			if err != nil {
				closeStatus := websocket.CloseStatus(err)
				if closeStatus != websocket.StatusNormalClosure &&
					closeStatus != websocket.StatusGoingAway &&
					closeStatus != websocket.StatusNoStatusRcvd {
					// 只記錄異常斷線，正常斷線由 Unregister 統一記錄
					logger.Error("Connection read error",
						zap.String("remoteAddr", r.RemoteAddr),
						zap.Error(err),
					)
				}
				break
			}
			if typ != websocket.MessageBinary {
				continue
			}

			data, err := io.ReadAll(msg)
			if err != nil {
				break
			}

			// 解析 Pack 消息
			var req gatepb.Pack
			if err := proto.Unmarshal(data, &req); err != nil {
				wsCtx.Throw(buildErrorMessage("protobuf 解析失敗", err))
				continue
			}

			// 清空上一次請求的錯誤狀態（避免錯誤訊息被帶到下一個請求）
			wsCtx.Set(webcontext.Error, "")

			// 生成 traceId（uid-reqId 格式），在收到封包的那一刻建立
			uid := wsCtx.GetUID()
			traceId := fmt.Sprintf("%d-%d", uid, req.ReqId)
			wsCtx.SetExtra("trace_id", traceId)

			// 保留原始 Pack，供後續轉發時使用
			wsCtx.SetExtra("raw_pack", &req)

			// 設置請求信息到上下文
			wsCtx.Set(webcontext.PackType, req.PackType) // 存儲 packType (0: request, 1: response 2: notify 3: trigger, 4:fetch )
			wsCtx.Set(webcontext.ReqID, req.ReqId)

			// 直接存儲 svt 和 method 到 context（使用專用字段，不使用 SetExtra）
			svt := req.GetSvt()
			method := req.GetMethod()
			wsCtx.Set(webcontext.Svt, svt)
			wsCtx.Set(webcontext.PackMethod, method)

			// 未認證連線僅允許 packType=0 (request)，由 Gate 的 Dispatcher 處理 auth/validate 或回錯
			if wsCtx.GetUID() == 0 && req.PackType != 0 {
				if wc := GetWSConn(wsCtx); wc != nil {
					_ = wc.Close(websocket.StatusPolicyViolation, "unauthorized")
				}
				break
			}

			// 驗證 svt 和 method
			if svt == "" || method == "" {
				route := svt + "/" + method // 用於錯誤訊息
				wsCtx.Throw("Svt and method are required")
				// 設置 Route 到 context（用於日誌和錯誤處理）
				wsCtx.Set(webcontext.Route, route)
				autoResponseMiddleware(wsCtx, func(ctx WSContext) *anypb.Any { return nil })
				continue
			}

			// 構建 route 字符串（僅用於日誌和錯誤訊息）
			route := svt + "/" + method
			wsCtx.Set(webcontext.Route, route)

			// 自動解析 Any 類型的請求數據
			if req.Info != nil {
				msg, err := webcontext.UnmarshalAnyDynamic(req.Info)
				if err == nil {
					wsCtx.Set(webcontext.ReqData, msg) // 存儲解析後的 proto.Message（使用專用字段）
				} else {
					wsCtx.Set(webcontext.ReqData, req.Info) // fallback，存儲 Any（使用專用字段）
				}
			} else {
				// req.Info 為 nil，這是允許的（某些 API 不需要請求參數）
				logger.Debug("Request has no info (empty request payload)",
					zap.String("svt", svt),
					zap.String("method", method),
					zap.String("route", route),
					zap.Uint32("reqID", req.ReqId),
				)
			}

			// 使用 svt + method 進行路由匹配（兩層索引，O(1) 查找）
			handlerInfo, ok := GetHandler(svt, method, req.PackType)

			if !ok {
				// 統一回應 error pack，方便定位 route 問題
				wsCtx.Throw(buildRouteErrorMessage("No handler for route:", route))
				autoResponseMiddleware(wsCtx, func(ctx WSContext) *anypb.Any { return nil })
				continue
			}

			// 準備 payload
			var payload proto.Message
			if handlerInfo.Ctor != nil {
				payload = handlerInfo.Ctor()
				if req.Info != nil {
					_ = req.Info.UnmarshalTo(payload)
				}
			}

			// 構建中間件鏈
			var finalHandler ResponseHandlerFunc
			isNotifyRequest := req.PackType == 2 || req.PackType == 3
			if isNotifyRequest {
				finalHandler = func(ctx WSContext) *anypb.Any {
					if handlerInfo.NotifyHandle != nil {
						if err := handlerInfo.NotifyHandle(ctx, ctx.GetUID(), &req, payload); err != nil {
							ctx.Throw(err.Error())
						}
					}
					return nil
				}
			} else {
			finalHandler = func(ctx WSContext) *anypb.Any {
				if handlerInfo.RequestHandle == nil {
					ctx.Throw("handler not implemented")
					return nil
				}
				resp, err := handlerInfo.RequestHandle(ctx, ctx.GetUID(), &req, payload)
				if err != nil {
					// 解析 gRPC error，提取 msg 和 detail
					msg, detail := svcerr.FromGRPCError(err)
					ctx.Throw(msg)
					if detail != "" {
						ctx.SetExtra("_error_detail", detail)
					}
					return nil
				}
				return resp
			}
			}

			// 組裝中間件鏈（從後往前）
			h := finalHandler
			for i := len(s.Middleware) - 1; i >= 0; i-- {
				mw := s.Middleware[i]
				next := h
				h = func(ctx WSContext) *anypb.Any {
					return mw(ctx, next)
				}
			}

			// 內建的自動回應中間件置於最外層
			builtIn := func(ctx WSContext) *anypb.Any {
				return autoResponseMiddleware(ctx, h)
			}

			// 執行處理鏈（包含內建 auto middleware）
			builtIn(wsCtx)
		}
	}
}

// GetCallMethodName 從 packType 獲取呼叫方法名稱
func GetCallMethodName(packType uint32) string {
	switch packType {
	case 0:
		return "request"
	case 2:
		return "notify"
	case 3:
		return "trigger"
	case 4:
		return "fetch"
	default:
		return "request"
	}
}

// autoResponseMiddleware: 統一捕獲 panic，並依 packType 回應或推播錯誤；同時處理成功回應。
func autoResponseMiddleware(ctx WSContext, next ResponseHandlerFunc) *anypb.Any {
	reqID := ctx.GetReqID()
	packType := ctx.GetUint32(webcontext.PackType)
	route := ctx.GetRoute()
	callMethod := GetCallMethodName(packType)
	fullRoute := fmt.Sprintf("%s/%s", callMethod, route)

	// 取得 traceId 與 uid（供日誌前綴）
	traceId := ""
	if t, ok := ctx.GetExtra("trace_id"); ok {
		traceId = t.(string)
	}
	logPrefix := logger.GateLogPrefix(ctx.GetUID(), traceId)

	var result *anypb.Any

	defer func() {
		if err := recover(); err != nil {
			panicMsg := fmt.Sprintf("%v", err)
			if packType == 2 || packType == 3 { // notify/trigger
				sendNotifyErrorPush(ctx, panicMsg)
			} else {
				logger.GateError(fmt.Sprintf("%s[◀][%s] Panic: %s", logPrefix, fullRoute, panicMsg))
				sendErrorResponse(ctx, route, reqID, panicMsg)
			}
			result = nil
		}
	}()

	// 執行 handler
	result = next(ctx)

	// 錯誤處理：統一使用 context.Error
	if errMsg, ok := ctx.Get(webcontext.Error); ok {
		errStr := errMsg.(string)
		if errStr != "" {
			if packType == 2 || packType == 3 { // notify/trigger -> 推播
				sendNotifyErrorPush(ctx, errStr)
			} else { // request/fetch -> 回應
				// 錯誤日誌已在 LoggingMiddleware 中記錄，這裡只發送回應
				sendErrorResponse(ctx, route, reqID, errStr)
			}
			return nil
		}
	}

	// 成功回應
	if result != nil {
		// 計算總處理時間
		durationStr := ""
		if startTime, ok := ctx.GetExtra("_req_start_time"); ok {
			if start, ok := startTime.(time.Time); ok {
				duration := time.Since(start)
				durationStr = fmt.Sprintf("[%v]", duration)
			}
		}
		// 解析 Any 以便日誌輸出
		var logMsg interface{} = result
		if result != nil {
			// 嘗試解析 Any 為實際的 proto.Message
			unmarshaled, err := webcontext.UnmarshalAnyDynamic(result)
			if err == nil && unmarshaled != nil {
				// 使用 protojson 格式化輸出，更易讀
				marshaler := protojson.MarshalOptions{
					UseProtoNames:   true,
					EmitUnpopulated: true,
					Multiline:       false,
				}
				if jsonBytes, jsonErr := marshaler.Marshal(unmarshaled); jsonErr == nil {
					logMsg = fmt.Sprintf("[%s]:%s", result.TypeUrl, string(jsonBytes))
				} else {
					// JSON 序列化失敗，使用默認格式
					logMsg = fmt.Sprintf("[%s]:%+v", result.TypeUrl, unmarshaled)
				}
			} else {
				// 如果解析失敗，嘗試直接使用 protojson 序列化 Any 本身
				// 這樣至少能看到 type_url，雖然 value 是二進制數據
				marshaler := protojson.MarshalOptions{
					UseProtoNames:   true,
					EmitUnpopulated: true,
				}
				if jsonBytes, jsonErr := marshaler.Marshal(result); jsonErr == nil {
					// 成功序列化，顯示 JSON（包含 type_url 和 base64 編碼的 value）
					logMsg = string(jsonBytes)
				} else {
					// 序列化也失敗，只顯示基本信息
					logMsg = fmt.Sprintf("{\"@type\":\"%s\",\"value\":\"<binary data, len=%d>\"}", result.TypeUrl, len(result.Value))
				}
				// 只在 Debug 級別記錄詳細錯誤（避免日誌過多）
				if err != nil {
					logger.Debug("Failed to unmarshal Any for logging, using Any JSON instead",
						zap.String("typeUrl", result.TypeUrl),
						zap.String("error", err.Error()),
						zap.Int("valueLen", len(result.Value)),
					)
				}
			}
		}
		// 略過 gate/ping 的 response log，以免干擾 debug
		if route != "gate/ping" {
			logger.GateInfo(fmt.Sprintf("%s[◀][%s]%s %v", logPrefix, fullRoute, durationStr, logMsg))
		}
		if err := sendSuccessResponse(ctx, reqID, result); err != nil {
			logger.GateError(fmt.Sprintf("%s Failed to send success response: %v", logPrefix, err))
		}
	}

	return result
}

// sendErrorResponse 發送錯誤響應
func sendErrorResponse(ctx WSContext, route string, reqID int32, errMsg string) {
	// 嘗試獲取錯誤細節
	detail := ""
	if d, ok := ctx.GetExtra("_error_detail"); ok {
		if detailStr, ok := d.(string); ok {
			detail = detailStr
		}
	}

	pack := common.Builder.BuildErrorPackWithDetail(1, route, reqID, errMsg, detail)
	
	// 設置 svt 和 method（從 context 獲取）
	svt := ctx.GetString(webcontext.Svt)
	method := ctx.GetString(webcontext.PackMethod)
	pack.Svt = svt
	pack.Method = method

	data, err := proto.Marshal(pack)
	if err != nil {
		logPrefix := getLogPrefix(ctx)
		logger.GateError(fmt.Sprintf("%s Failed to marshal error response: %v", logPrefix, err))
		return
	}

	SendMessageOrLog(ctx, data, "")
}

// sendSuccessResponse 發送成功響應
func sendSuccessResponse(ctx WSContext, reqID int32, respMsg *anypb.Any) error {
	route := ctx.GetRoute()
	logPrefix := getLogPrefix(ctx)

	pack, err := common.Builder.BuildSuccessPack(1, reqID, route, respMsg)
	if err != nil {
		logger.GateError(fmt.Sprintf("%s Failed to build success pack: %v", logPrefix, err))
		sendErrorResponse(ctx, route, reqID, "Failed to create response")
		return err
	}
	
	// 設置 svt 和 method（從 context 獲取）
	svt := ctx.GetString(webcontext.Svt)
	method := ctx.GetString(webcontext.PackMethod)
	pack.Svt = svt
	pack.Method = method

	data, err := proto.Marshal(pack)
	if err != nil {
		logger.GateError(fmt.Sprintf("%s Failed to marshal response: %v", logPrefix, err))
		return err
	}

	return SendMessageOrLog(ctx, data, "")
}

// getLogPrefix 從 WSContext 取得 uid 與 trace_id，組出 [uid:xx | traceID:xx] 前綴
func getLogPrefix(ctx WSContext) string {
	traceId := ""
	if t, ok := ctx.GetExtra("trace_id"); ok {
		traceId = t.(string)
	}
	return logger.GateLogPrefix(ctx.GetUID(), traceId)
}

// sendNotifyErrorPush 發送 notify 錯誤推送
func sendNotifyErrorPush(ctx WSContext, errMsg string) {
	route := "server/error"
	logPrefix := getLogPrefix(ctx)

	pack := common.Builder.BuildNotifyErrorPack(route, errMsg)
	
	// 從 route 解析 svt 和 method
	parts := strings.Split(route, "/")
	if len(parts) == 2 {
		pack.Svt = parts[0]
		pack.Method = parts[1]
	}

	data, err := proto.Marshal(pack)
	if err != nil {
		logger.GateError(fmt.Sprintf("%s Failed to marshal notify error push: %v", logPrefix, err))
		return
	}

	if SendMessageOrLog(ctx, data, "") == nil {
		logger.GateInfo(fmt.Sprintf("%s[◀][%s] NotifyError: %s", logPrefix, ctx.GetRoute(), errMsg))
	}
}

// pingLoop 定期發送 ping 來保持連接活躍
func (s *Server) pingLoop(ctx stdcontext.Context, conn *websocket.Conn, remoteAddr string) {
	ticker := time.NewTicker(s.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 創建帶超時的 context 用於 ping
			pingCtx, cancel := stdcontext.WithTimeout(ctx, s.PingTimeout)
			err := conn.Ping(pingCtx)
			cancel()

			if err != nil {
				logger.Warn("Ping failed",
					zap.String("remoteAddr", remoteAddr),
					zap.Error(err),
				)
				// Ping 失敗，通常意味著連接已經斷開
				// Reader 會自動處理連接關閉
				return
			}
			logger.Debug("Ping successful",
				zap.String("remoteAddr", remoteAddr),
			)
		}
	}
}

// Start 啟動 WebSocket 服務
func (s *Server) Start() {
	if err := http.ListenAndServe(buildServerAddress(s.Port), nil); err != nil {
		logger.Fatal("WebSocket server failed to start",
			zap.String("port", s.Port),
			zap.Error(err),
		)
	}
}
