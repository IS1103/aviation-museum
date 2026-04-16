package grpc

import (
	"fmt"
	"os"
	"sync"

	"internal/etcd"
	"internal/logger"

	"google.golang.org/grpc"
)

var (
	globalPool      *ConnectionPool
	globalPoolOnce  sync.Once
	globalDiscovery Discovery // 全局服務發現器（etcd）
	discoveryMu     sync.RWMutex
	lazyWatched     map[string]bool // 已啟動 Watch 的服務名（lazy 發現用）
	lazyWatchedMu   sync.Mutex
	connectMuxMap   sync.Map // serviceName -> *sync.Mutex，避免並發時重複 Connect 同一服務導致 CONNECTING 競態
)

// getConnectMutex 回傳該服務專用的 mutex，確保同一時間只有一個 Connect 在執行。
func getConnectMutex(serviceName string) *sync.Mutex {
	v, _ := connectMuxMap.LoadOrStore(serviceName, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// InitGRPC 初始化 gRPC 服務發現（僅註冊自己到 etcd）與連接池。
// 其他服務的連線改由首次呼叫 PickConnection(serviceName) 時從 etcd 動態發現（lazy）。
// serviceName: 當前服務名稱；instanceID: 當前實例 ID；grpcPort: gRPC 端口；configDiscoveryMode: 可選 discovery_mode。
func InitGRPC(serviceName, instanceID, grpcPort string, _ map[string]string, configDiscoveryMode ...string) error {
	globalPoolOnce.Do(func() {
		globalPool = NewConnectionPool()
	})
	if lazyWatched == nil {
		lazyWatched = make(map[string]bool)
	}

	var configMode string
	if len(configDiscoveryMode) > 0 && configDiscoveryMode[0] != "" {
		configMode = configDiscoveryMode[0]
	}
	mode := GetDiscoveryMode(configMode)

	if mode == DiscoveryModeEtcd {
		etcdClient := etcd.Default()
		if etcdClient == nil || etcdClient.Cli == nil {
			etcd.MustInitFromEnv()
		}
	}

	if instanceID == "" {
		instanceID = GetInstanceID(serviceName)
	}
	serviceIP := os.Getenv("SERVICE_IP")
	if serviceIP == "" {
		serviceIP = "localhost"
	}

	config := DiscoveryConfig{
		ServiceName: serviceName,
		InstanceID:  instanceID,
		GRPCPort:    grpcPort,
		ServiceIP:   serviceIP,
	}
	discovery, err := NewDiscovery(mode, config)
	if err != nil {
		return fmt.Errorf("初始化服務發現失敗: %w", err)
	}

	discoveryMu.Lock()
	globalDiscovery = discovery
	discoveryMu.Unlock()

	if mode == DiscoveryModeEtcd {
		etcdEndpoints := os.Getenv("ETCD_ENDPOINTS")
		if etcdEndpoints == "" {
			etcdEndpoints = "localhost:2379"
		}
		logger.GateInfof("etcd 已初始化 (endpoints: %s)，本服務已註冊", etcdEndpoints)
	}
	return nil
}

// GetPool 獲取全局連接池
func GetPool() *ConnectionPool {
	globalPoolOnce.Do(func() {
		globalPool = NewConnectionPool()
	})
	return globalPool
}

// ensureLazyWatch 對 serviceName 確保已啟動 Watch（僅執行一次），port 傳 "" 表示使用 etcd 內存之完整地址。
func ensureLazyWatch(discovery Discovery, serviceName string) {
	lazyWatchedMu.Lock()
	if lazyWatched[serviceName] {
		lazyWatchedMu.Unlock()
		return
	}
	lazyWatched[serviceName] = true
	lazyWatchedMu.Unlock()

	_ = discovery.Watch(serviceName, "", func(instances []string) {
		if len(instances) > 0 {
			logger.Debugf("[Watch] %s 實例列表更新: %v", serviceName, instances)
		}
	})
}

// PickConnection 選擇一個連接。首次使用某服務時從 etcd 發現並建立連線（lazy）。
// 同一服務並發呼叫時會串行化 Connect，避免回傳 CONNECTING 連線導致 RPC 失敗。
func PickConnection(serviceName string) (*grpc.ClientConn, error) {
	logger.Debugf("🔍 [PickConnection] 開始選擇連接: %s", serviceName)

	discoveryMu.RLock()
	discovery := globalDiscovery
	discoveryMu.RUnlock()

	if discovery != nil {
		ensureLazyWatch(discovery, serviceName)
		mu := getConnectMutex(serviceName)
		mu.Lock()
		defer mu.Unlock()
		conn, err := discovery.GetConnection(serviceName, "")
		if err == nil {
			logger.Debugf("✅ [PickConnection] 從 discovery 獲取連接: %s", serviceName)
			return conn, nil
		}
		conn, err = discovery.Connect(serviceName, "", "")
		if err != nil {
			logger.Warnf("⚠️ [PickConnection] 連接失敗: %s, %v", serviceName, err)
			return nil, err
		}
		logger.Infof("grpc connected: %s (%v)", serviceName, conn.GetState())
		return conn, nil
	}

	conn, err := GetPool().Pick(serviceName)
	if err != nil {
		logger.Warnf("⚠️ [PickConnection] 從連接池獲取失敗: %s, %v", serviceName, err)
	} else {
		logger.Debugf("✅ [PickConnection] 從連接池獲取: %s", serviceName)
	}
	return conn, err
}

// PickConnectionWithInstanceID 選擇一個連接並回傳實例 ID（用於 Match CAS 寫入 sid）。若僅有 discovery 無 pool 則 instanceID 為空。
func PickConnectionWithInstanceID(serviceName string) (*grpc.ClientConn, string, error) {
	conn, instanceID, err := GetPool().PickWithInstanceID(serviceName)
	if err == nil && conn != nil {
		return conn, instanceID, nil
	}
	conn, err = PickConnection(serviceName)
	if err != nil {
		return nil, "", err
	}
	return conn, "", nil
}

// PickConnectionByInstance 根據實例 ID 選擇連接。從 etcd 取得該實例地址後建立連線（port 傳 ""）。
func PickConnectionByInstance(serviceName, instanceID string) (*grpc.ClientConn, error) {
	if instanceID == "" {
		return PickConnection(serviceName)
	}

	pool := GetPool()
	if conn, err := pool.GetConnection(serviceName, instanceID); err == nil && conn != nil {
		state := conn.GetState()
		if state.String() == "READY" || state.String() == "IDLE" {
			return conn, nil
		}
		_ = conn.Close()
		pool.Remove(serviceName, instanceID)
	}

	discoveryMu.RLock()
	discovery := globalDiscovery
	discoveryMu.RUnlock()

	if discovery != nil {
		conn, err := discovery.GetConnection(serviceName, instanceID)
		if err == nil {
			pool.Add(serviceName, instanceID, conn)
			return conn, nil
		}
		conn, err = discovery.Connect(serviceName, instanceID, "")
		if err == nil {
			pool.Add(serviceName, instanceID, conn)
			logger.Infof("✅ 動態建立連接到服務實例: %s/%s", serviceName, instanceID)
			return conn, nil
		}
	}
	return pool.GetConnection(serviceName, instanceID)
}

// CloseGRPC 關閉所有 gRPC 連接和服務發現
func CloseGRPC() {
	// 關閉服務發現器
	discoveryMu.Lock()
	if globalDiscovery != nil {
		_ = globalDiscovery.Close()
		globalDiscovery = nil
	}
	discoveryMu.Unlock()

	// 關閉連接池
	if globalPool != nil {
		globalPool.Close()
	}

	logger.Info("✅ gRPC 服務已關閉")
}
