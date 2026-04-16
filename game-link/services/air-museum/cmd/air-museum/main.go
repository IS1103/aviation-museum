package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"air-museum/config"
	"air-museum/internal/api"
	"air-museum/internal/db"
	"air-museum/internal/handler"

	"internal/gateforward"
	"internal/logger"
	wsmiddleware "internal/middleware/ws"
	"internal/webcore/ws"
)

func main() {
	config.Load()

	pgDBExisted, err := db.Init(context.Background())
	if err != nil {
		if config.GetPostgresDSN() != "" {
			logger.GateFatalf("postgres 初始化失敗: %v", err)
		}
		logger.GateWarnf("postgres init skipped or failed: %v", err)
	} else if pgDBExisted {
		logger.GateInfo("[postgres] 目標資料庫在啟動前已存在，略過 CREATE DATABASE")
	}
	defer db.Close()

	// 直連：本機分發 request/notify/trigger，不轉發到其他 service
	gateforward.DefaultDispatcher = gateforward.NewForwardRegistryDispatcher("air_museum")
	gateforward.DefaultNotifyDispatcher = gateforward.NewForwardNotifyRegistryDispatcher("air_museum")
	gateforward.DefaultTriggerDispatcher = gateforward.NewForwardTriggerRegistryDispatcher("air_museum")

	handler.RegisterDisconnectCleanup()

	wsPort := fmt.Sprintf("%d", config.GetWSPort())
	wsServer := ws.NewServer(wsPort, wsmiddleware.LoggingMiddleware)
	ws.ApplyRoutes(wsServer)

	httpPort := config.GetHTTPPort()
	httpAddr := fmt.Sprintf(":%d", httpPort)
	ginRouter := api.SetupRouter()
	httpServer := &http.Server{Addr: httpAddr, Handler: ginRouter}

	logger.LogServiceInit(logger.ServiceInitConfig{
		ServiceName: "air-museum",
		Fields: map[string]string{
			"svt":   config.GetSvt(),
			"sid":   config.GetServiceID(),
			"WS":    wsPort,
			"HTTP":  httpAddr,
			"mode":  "direct",
		},
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		wsServer.Start()
	}()
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.GateWarnf("HTTP server error: %v", err)
		}
	}()

	sig := <-quit
	logger.GateInfof("收到終止信號: %v，正在關閉服務...", sig)
	_ = httpServer.Shutdown(context.Background())
	logger.GateInfo("air-museum WS 已停止")
	os.Exit(0)
}
