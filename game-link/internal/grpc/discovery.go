package grpc

import (
	"google.golang.org/grpc"
)

// Discovery 服務發現接口
// etcd 服務發現方式的接口
type Discovery interface {
	// GetAddress 獲取服務地址（用於建立連接）
	// serviceName: 服務名稱，例如 "profile", "gate"
	// instanceID: 實例 ID（可選，用於指定特定實例）
	// port: 服務端口
	// 返回完整地址，例如 "profile:50054" 或 "192.168.1.1:50054"
	GetAddress(serviceName, instanceID, port string) (string, error)

	// GetInstances 獲取所有實例地址（用於負載均衡）
	// serviceName: 服務名稱
	// port: 服務端口
	// 返回所有實例的地址列表
	GetInstances(serviceName, port string) ([]string, error)

	// Watch 監聽服務變化
	// serviceName: 服務名稱
	// port: 服務端口
	// callback: 當服務列表變化時的回調函數
	// 返回錯誤或 nil
	Watch(serviceName, port string, callback func([]string)) error

	// Register 註冊當前服務
	// serviceName: 服務名稱
	// instanceID: 實例 ID
	// address: 服務地址（IP:Port）
	// 返回錯誤或 nil
	Register(serviceName, instanceID, address string) error

	// Unregister 取消註冊服務
	// serviceName: 服務名稱
	// instanceID: 實例 ID
	// 返回錯誤或 nil
	Unregister(serviceName, instanceID string) error

	// Connect 建立 gRPC 連接
	// serviceName: 服務名稱
	// instanceID: 實例 ID（可選）
	// port: 服務端口
	// 返回 gRPC 連接
	Connect(serviceName, instanceID, port string) (*grpc.ClientConn, error)

	// GetConnection 獲取已建立的連接
	// serviceName: 服務名稱
	// instanceID: 實例 ID（可選）
	// 返回 gRPC 連接
	GetConnection(serviceName, instanceID string) (*grpc.ClientConn, error)

	// Close 關閉所有連接和資源
	Close() error
}

