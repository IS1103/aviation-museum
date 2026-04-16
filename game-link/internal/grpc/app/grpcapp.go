package grpcapp

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	grpcdisc "internal/grpc"
	"internal/logger"

	gatepb "internal.proto/pb/gate"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
)

// formatPackPayload 將 Pack 的 info (Any) 解出並格式化為 JSON 字串，供日誌顯示。
func formatPackPayload(pack *gatepb.Pack) string {
	if pack == nil || pack.GetInfo() == nil {
		return ""
	}
	any := pack.GetInfo()
	mt, err := protoregistry.GlobalTypes.FindMessageByURL(any.GetTypeUrl())
	if err != nil {
		return fmt.Sprintf("[type:%s err:%v]", any.GetTypeUrl(), err)
	}
	msg := mt.New().Interface()
	if err := proto.Unmarshal(any.GetValue(), msg); err != nil {
		return fmt.Sprintf("[type:%s unmarshal_err:%v]", any.GetTypeUrl(), err)
	}
	jsonBytes, err := protojson.Marshal(msg)
	if err != nil {
		return fmt.Sprintf("[json_err:%v]", err)
	}
	s := string(jsonBytes)
	if len(s) > 512 {
		s = s[:512] + "..."
	}
	return s
}

// formatResponseAny 將 Fetch 回傳的 *anypb.Any 解出並格式化為 JSON 字串，供日誌顯示。
func formatResponseAny(a *anypb.Any) string {
	if a == nil || a.GetTypeUrl() == "" {
		return ""
	}
	mt, err := protoregistry.GlobalTypes.FindMessageByURL(a.GetTypeUrl())
	if err != nil {
		return fmt.Sprintf("[type:%s err:%v]", a.GetTypeUrl(), err)
	}
	msg := mt.New().Interface()
	if err := proto.Unmarshal(a.GetValue(), msg); err != nil {
		return fmt.Sprintf("[type:%s unmarshal_err:%v]", a.GetTypeUrl(), err)
	}
	jsonBytes, err := protojson.Marshal(msg)
	if err != nil {
		return fmt.Sprintf("[json_err:%v]", err)
	}
	s := string(jsonBytes)
	if len(s) > 1024 {
		s = s[:1024] + "..."
	}
	return s
}

// RuntimeConfig 服務運行時配置（傳入 Run/RunGame 的統一型別，由各服務從自身 config 組裝）。
type RuntimeConfig struct {
	ServiceName   string // 服務類型（svt）
	SID           string // 服務實例 ID
	Port          string // gRPC 監聽端口
	ServiceIP     string // 註冊到 etcd 的 IP（可選）
	DiscoveryMode string // 服務發現模式："etcd"（默認 "etcd"）
}

// Config 定義共用 gRPC 服務啟動配置
type Config struct {
	RuntimeConfig RuntimeConfig
	Register      func(s *grpc.Server)
	// BeforeListen 在 InitGRPC 之後、Listen 之前呼叫；若回傳錯誤則 Run 失敗（用於 Game 拉取 GetGameInfo 等）。
	BeforeListen func() error
	OnShutdown   func() // 收到 SIGINT/SIGTERM 時，在 GracefulStop 前呼叫
}

// loggingInterceptor 統一的 gRPC 請求/響應日誌攔截器
func loggingInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()

		// 從 metadata 提取 trace_id、uid、caller、route（method）
		traceId := ""
		uid := ""
		caller := ""
		route := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get("trace_id"); len(vals) > 0 {
				traceId = vals[0]
			}
			if vals := md.Get("uid"); len(vals) > 0 {
				uid = vals[0]
			}
			if vals := md.Get("x-caller-svt"); len(vals) > 0 {
				caller = vals[0]
			}
			if vals := md.Get("method"); len(vals) > 0 {
				route = vals[0]
			}
		}
		if caller == "" {
			caller = "-"
		}
		// route：Request/Notify 從 req.Pack.Method 取；Trigger/Fetch 已從 metadata 取
		if route == "" {
			if pack, ok := req.(*gatepb.Pack); ok {
				route = pack.GetMethod()
			}
			if route == "" {
				route = "-"
			}
		}
		logPrefix := logger.GateLogPrefix(uid, traceId)

		// gRPC 方法名：FullMethod 最後一段，例如 /gate.GateService/Request → Request
		grpcMethod := strings.TrimPrefix(info.FullMethod, "/")
		if idx := strings.LastIndex(grpcMethod, "/"); idx >= 0 {
			grpcMethod = grpcMethod[idx+1:]
		}

		// 記錄請求：grpc/{caller}/{receiver}/{grpcMethod}/{route} — 誰發來、發給誰、哪個 API、哪個 handler
		logLine := fmt.Sprintf("grpc/%s/%s/%s/%s", caller, serviceName, grpcMethod, route)
		if pack, ok := req.(*gatepb.Pack); ok {
			if payload := formatPackPayload(pack); payload != "" {
				logLine += " " + payload
			}
		}
		logger.GateInfo(fmt.Sprintf("%s[▶][%s]", logPrefix, logLine))

		// 執行實際處理
		resp, err := handler(ctx, req)

		// 計算耗時
		duration := time.Since(startTime)
		durationStr := fmt.Sprintf("[%.2fms]", float64(duration.Microseconds())/1000)

		// 記錄響應
		if err != nil {
			st, _ := status.FromError(err)
			logger.GateError(fmt.Sprintf("%s[◀][%s]%s Error: %s", logPrefix, logLine, durationStr, st.Message()))
		} else {
			respInfo := ""
			if a, ok := resp.(*anypb.Any); ok && a != nil {
				if payload := formatResponseAny(a); payload != "" {
					respInfo = " " + payload
				}
			}
			logger.GateInfo(fmt.Sprintf("%s[◀][%s]%s ok%s", logPrefix, logLine, durationStr, respInfo))
		}

		return resp, err
	}
}

// Run 啟動 gRPC 服務（含 etcd 服務發現、信號優雅關閉）
func Run(cfg Config) error {
	r := &cfg.RuntimeConfig
	if r.ServiceName == "" || r.SID == "" || r.Port == "" {
		return fmt.Errorf("grpcapp: RuntimeConfig.ServiceName/SID/Port 不能為空")
	}
	if r.ServiceIP != "" {
		os.Setenv("SERVICE_IP", r.ServiceIP)
	}

	// 輸出服務初始化日誌
	serviceIP := r.ServiceIP
	if serviceIP == "" {
		serviceIP = "localhost"
	}
	logger.LogServiceInit(logger.ServiceInitConfig{
		ServiceName: r.ServiceName,
		Fields: map[string]string{
			"svt":   r.ServiceName,
			"sid":   r.SID,
			"gRPC":  r.Port,
			"IP":    serviceIP,
		},
	})

	// 初始化 gRPC 服務發現（etcd 模式）
	if err := grpcdisc.InitGRPC(r.ServiceName, r.SID, r.Port, nil, r.DiscoveryMode); err != nil {
		return fmt.Errorf("grpcapp: InitGRPC 失敗: %w", err)
	}
	defer grpcdisc.CloseGRPC()

	if cfg.BeforeListen != nil {
		if err := cfg.BeforeListen(); err != nil {
			return fmt.Errorf("grpcapp: BeforeListen 失敗: %w", err)
		}
	}

	// 建立 gRPC 服務（帶日誌攔截器）
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor(r.ServiceName)),
	)
	if cfg.Register != nil {
		cfg.Register(grpcServer)
	}

	lis, err := net.Listen("tcp", ":"+r.Port)
	if err != nil {
		return fmt.Errorf("grpcapp: 監聽端口失敗: %w", err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.GateFatalf("gRPC 伺服器啟動失敗: %v", err)
		}
	}()

	sig := <-quit
	logger.GateInfof("收到終止信號: %v，正在關閉服務...", sig)
	if cfg.OnShutdown != nil {
		cfg.OnShutdown()
	}
	grpcServer.GracefulStop()
	logger.GateInfo("gRPC 服務已停止")
	return nil
}
