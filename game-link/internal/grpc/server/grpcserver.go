package grpcserver

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	internalgrpc "internal/grpc"

	googlegrpc "google.golang.org/grpc"
)

// GRPCInitOptions 僅包含註冊到 etcd 所需參數；其他服務改由 PickConnection 時從 etcd 動態發現。
type GRPCInitOptions struct {
	Svt           string
	Sid           string
	Port          string
	ServiceIP     string
	DiscoveryMode string // 服務發現模式："etcd"（默認 "etcd"）
}

// InitWithK8sAndPool 初始化 etcd 註冊與 gRPC 連接池，返回 cleanup 函數。
func InitWithK8sAndPool(opts GRPCInitOptions) (func(), error) {
	cleanup := func() {
		internalgrpc.CloseGRPC()
	}
	if err := internalgrpc.InitGRPC(opts.Svt, opts.Sid, opts.Port, nil, opts.DiscoveryMode); err != nil {
		cleanup()
		return nil, err
	}
	return cleanup, nil
}

// Start starts a gRPC server on given port and registers services via registrar.
// It returns the server and listener for further control.
// Note: The server is started in a goroutine, but net.Listen immediately binds the port.
// Callers should wait a brief moment after calling Start before registering to service discovery
// to ensure the gRPC server is fully ready to accept connections.
func Start(port string, registrar func(*googlegrpc.Server)) (*googlegrpc.Server, net.Listener, error) {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, nil, err
	}
	s := googlegrpc.NewServer()
	registrar(s)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Printf("gRPC serve error: %v", err)
		}
	}()
	return s, lis, nil
}

// WaitForSignal blocks until SIGINT/SIGTERM then gracefully stops server and closes listener.
// onShutdown 可選，收到信號時在 GracefulStop 前呼叫（例如刪除 Redis 綁定）。
func WaitForSignal(s *googlegrpc.Server, lis net.Listener, onShutdown func()) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("🛑 收到終止信號: %v，正在關閉 gRPC 服務...", sig)

	if onShutdown != nil {
		onShutdown()
	}
	if s != nil {
		s.GracefulStop()
	}
	if lis != nil {
		_ = lis.Close()
	}
	log.Println("✅ gRPC 服務已停止")
}
