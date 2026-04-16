package grpc

import (
	"fmt"
	"os"
	"strings"

	"internal/logger"
)

// DiscoveryMode 服務發現模式
type DiscoveryMode string

const (
	// DiscoveryModeEtcd etcd 服務發現模式
	DiscoveryModeEtcd DiscoveryMode = "etcd"
)

// DiscoveryConfig 服務發現配置（僅用於註冊自己到 etcd）
type DiscoveryConfig struct {
	ServiceName string // 當前服務名稱
	InstanceID  string // 當前實例 ID
	GRPCPort    string // gRPC 服務端口
	ServiceIP   string // 服務 IP（etcd 模式需要）
}

// NewDiscovery 創建服務發現器（工廠方法）
// mode: 服務發現模式（僅支持 "etcd"）
// config: 服務發現配置
func NewDiscovery(mode DiscoveryMode, config DiscoveryConfig) (Discovery, error) {
	switch mode {
	case DiscoveryModeEtcd:
		return newEtcdDiscovery(config)
	default:
		return nil, fmt.Errorf("不支持的服務發現模式: %s (僅支持 'etcd')", mode)
	}
}

// GetDiscoveryMode 獲取服務發現模式
// 優先順序：環境變量 > 配置檔 > 默認值
// configMode: 從配置檔讀取的模式（可選）
func GetDiscoveryMode(configMode string) DiscoveryMode {
	// 從環境變量獲取（優先）
	if mode := os.Getenv("DISCOVERY_MODE"); mode != "" {
		mode = strings.ToLower(mode)
		if mode == "etcd" {
			return DiscoveryModeEtcd
		}
		logger.Warnf("⚠️  環境變量 DISCOVERY_MODE 值無效: %s，使用配置檔或默認值", mode)
	}

	// 從配置檔獲取
	if configMode != "" {
		mode := strings.ToLower(configMode)
		if mode == "etcd" {
			return DiscoveryModeEtcd
		}
		logger.Warnf("⚠️  配置檔 discovery_mode 值無效: %s，使用默認值 etcd", configMode)
	}

	// 默認使用 etcd 模式
	return DiscoveryModeEtcd
}

// newEtcdDiscovery 創建 etcd 服務發現器
func newEtcdDiscovery(config DiscoveryConfig) (Discovery, error) {
	discovery, err := NewEtcdDiscovery()
	if err != nil {
		return nil, fmt.Errorf("創建 etcd 服務發現器失敗: %w", err)
	}

	// 僅註冊當前服務，不預先發現或連接其他服務（改由 PickConnection 時從 etcd lazy 發現）
	if config.ServiceName != "" && config.InstanceID != "" {
		serviceIP := config.ServiceIP
		if serviceIP == "" {
			serviceIP = "localhost"
		}
		address := fmt.Sprintf("%s:%s", serviceIP, config.GRPCPort)
		if err := discovery.Register(config.ServiceName, config.InstanceID, address); err != nil {
			return nil, fmt.Errorf("註冊服務到 etcd 失敗: %w", err)
		}
	}

	return discovery, nil
}

