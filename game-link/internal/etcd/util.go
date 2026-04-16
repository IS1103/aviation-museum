package etcd

import (
	"internal/logger"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

// MustInitFromEnv 從環境變量初始化 etcd 客戶端，失敗則 panic
func MustInitFromEnv() {
	endpoints := os.Getenv("ETCD_ENDPOINTS")
	if endpoints == "" {
		endpoints = "localhost:2379" // 默認值
	}

	endpointList := strings.Split(endpoints, ",")
	if err := Init(endpointList, 5*time.Second); err != nil {
		logger.Fatal("Failed to init etcd",
			zap.Error(err),
			zap.Strings("endpoints", endpointList),
		)
	}
}
