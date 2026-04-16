package grpc

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"internal/etcd"
	"internal/logger"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// etcd 服務註冊路徑前綴
	etcdServicePrefix = "/services"
	// 默認 TTL 30 秒
	defaultTTL = 30 * time.Second
	// 續約間隔 10 秒（TTL 的 1/3）
	leaseRenewInterval = 10 * time.Second
)

// EtcdDiscovery etcd 服務發現器
type EtcdDiscovery struct {
	etcdClient *etcd.Client
	leases     map[string]clientv3.LeaseID // serviceName:instanceID -> leaseID
	leasesMu   sync.RWMutex
	watchers   map[string]context.CancelFunc // serviceName -> cancelFunc
	watchersMu sync.RWMutex
	conns      map[string]map[string]*grpc.ClientConn // serviceName -> instanceID -> conn
	connsMu    sync.RWMutex
	roundRobin map[string]int // serviceName -> 當前索引
	rrMu       sync.Mutex
}

// NewEtcdDiscovery 創建 etcd 服務發現器
func NewEtcdDiscovery() (*EtcdDiscovery, error) {
	etcdClient := etcd.Default()
	if etcdClient == nil || etcdClient.Cli == nil {
		return nil, fmt.Errorf("etcd 客戶端未初始化，請先調用 etcd.Init() 或 etcd.MustInitFromEnv()")
	}

	return &EtcdDiscovery{
		etcdClient: etcdClient,
		leases:     make(map[string]clientv3.LeaseID),
		watchers:   make(map[string]context.CancelFunc),
		conns:      make(map[string]map[string]*grpc.ClientConn),
		roundRobin: make(map[string]int),
	}, nil
}

// GetAddress 獲取服務地址
// etcd 模式下，如果指定了 instanceID，返回該實例地址；否則返回第一個可用實例地址
func (d *EtcdDiscovery) GetAddress(serviceName, instanceID, port string) (string, error) {
	if instanceID != "" {
		// 從 etcd 讀取指定實例的地址
		key := d.getServiceKey(serviceName, instanceID)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := d.etcdClient.Cli.Get(ctx, key)
		if err != nil {
			return "", fmt.Errorf("從 etcd 讀取服務地址失敗: %w", err)
		}

		if len(resp.Kvs) == 0 {
			return "", fmt.Errorf("服務實例 %s/%s 不存在", serviceName, instanceID)
		}

		address := string(resp.Kvs[0].Value)
		// 若未帶端口且呼叫方有提供 port 才拼接
		if port != "" && !strings.Contains(address, ":") {
			address = fmt.Sprintf("%s:%s", address, port)
		}
		return address, nil
	}

	// 獲取所有實例，返回第一個
	instances, err := d.GetInstances(serviceName, port)
	if err != nil {
		return "", err
	}
	if len(instances) == 0 {
		return "", fmt.Errorf("服務 %s 沒有可用實例", serviceName)
	}
	return instances[0], nil
}

// GetInstances 獲取所有實例地址
func (d *EtcdDiscovery) GetInstances(serviceName, port string) ([]string, error) {
	keyPrefix := d.getServiceKeyPrefix(serviceName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := d.etcdClient.Cli.Get(ctx, keyPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("從 etcd 讀取服務列表失敗: %w", err)
	}

	instances := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		address := string(kv.Value)
		// 僅在 port 非空且地址未含 ":" 時才拼接（etcd 通常存完整 host:port）
		if port != "" && !strings.Contains(address, ":") {
			address = fmt.Sprintf("%s:%s", address, port)
		}
		instances = append(instances, address)
	}

	return instances, nil
}

// Watch 監聽服務變化
func (d *EtcdDiscovery) Watch(serviceName, port string, callback func([]string)) error {
	keyPrefix := d.getServiceKeyPrefix(serviceName)
	
	ctx, cancel := context.WithCancel(context.Background())

	// 保存 cancel 函數，用於停止監聽
	d.watchersMu.Lock()
	if oldCancel, exists := d.watchers[serviceName]; exists {
		oldCancel() // 停止舊的監聽
	}
	d.watchers[serviceName] = cancel
	d.watchersMu.Unlock()

	// 先獲取當前所有實例
	instances, err := d.GetInstances(serviceName, port)
	if err == nil && len(instances) > 0 {
		callback(instances)
	}

	// 啟動監聽
	go func() {
		// 追蹤上次的實例列表，用於比對變化
		lastInstances := make(map[string]bool)
		for _, addr := range instances {
			lastInstances[addr] = true
		}

		watchChan := d.etcdClient.Cli.Watch(ctx, keyPrefix, clientv3.WithPrefix())
		for watchResp := range watchChan {
			if watchResp.Canceled {
				return
			}

			// 重新獲取所有實例
			instances, err := d.GetInstances(serviceName, port)
			if err != nil {
				// 獲取實例失敗，跳過本次更新
				continue
			}

			// 比對變化並輸出日誌
			currentInstances := make(map[string]bool)
			for _, addr := range instances {
				currentInstances[addr] = true
			}

			// 檢查新增的服務
			for addr := range currentInstances {
				if !lastInstances[addr] {
					logger.GateInfof("服務上線: %s -> %s", serviceName, addr)
				}
			}

			// 檢查移除的服務
			for addr := range lastInstances {
				if !currentInstances[addr] {
					logger.GateInfof("服務下線: %s -> %s", serviceName, addr)
				}
			}

			// 更新上次的實例列表
			lastInstances = currentInstances

			// 更新連接池
			d.updateConnections(serviceName, instances)

			// 調用回調
			callback(instances)
		}
	}()

	return nil
}

// Register 註冊當前服務
func (d *EtcdDiscovery) Register(serviceName, instanceID, address string) error {
	key := d.getServiceKey(serviceName, instanceID)

	// 創建租約
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	leaseResp, err := d.etcdClient.Cli.Grant(ctx, int64(defaultTTL.Seconds()))
	if err != nil {
		return fmt.Errorf("創建 etcd 租約失敗: %w", err)
	}

	leaseID := leaseResp.ID

	// 將服務信息寫入 etcd
	_, err = d.etcdClient.Cli.Put(ctx, key, address, clientv3.WithLease(leaseID))
	if err != nil {
		return fmt.Errorf("註冊服務到 etcd 失敗: %w", err)
	}

	// 保存租約 ID
	d.leasesMu.Lock()
	d.leases[fmt.Sprintf("%s:%s", serviceName, instanceID)] = leaseID
	d.leasesMu.Unlock()

	logger.GateInfof("服務已註冊: %s/%s -> %s", serviceName, instanceID, address)

	// 啟動自動續約
	go d.keepAlive(serviceName, instanceID, leaseID)

	return nil
}

// Unregister 取消註冊服務
func (d *EtcdDiscovery) Unregister(serviceName, instanceID string) error {
	key := d.getServiceKey(serviceName, instanceID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := d.etcdClient.Cli.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("從 etcd 取消註冊服務失敗: %w", err)
	}

	// 移除租約
	d.leasesMu.Lock()
	delete(d.leases, fmt.Sprintf("%s:%s", serviceName, instanceID))
	d.leasesMu.Unlock()

	logger.Infof("服務已取消註冊: %s/%s", serviceName, instanceID)
	return nil
}

// waitForReady 等待連線進入 READY 或 IDLE，避免並發請求在 CONNECTING 時使用導致 RPC 失敗。
func waitForReady(conn *grpc.ClientConn, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		state := conn.GetState()
		switch state {
		case connectivity.Ready, connectivity.Idle:
			return nil
		case connectivity.Shutdown, connectivity.TransientFailure:
			return fmt.Errorf("connection in state %v", state)
		}
		if !conn.WaitForStateChange(ctx, state) {
			return ctx.Err()
		}
	}
}

// Connect 建立 gRPC 連接
func (d *EtcdDiscovery) Connect(serviceName, instanceID, port string) (*grpc.ClientConn, error) {
	const connectReadyTimeout = 10 * time.Second

	// 如果指定了 instanceID，連接到特定實例
	if instanceID != "" {
		address, err := d.GetAddress(serviceName, instanceID, port)
		if err != nil {
			return nil, err
		}

		conn, err := grpc.Dial(
			address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, fmt.Errorf("連接服務失敗: %w", err)
		}
		if err := waitForReady(conn, connectReadyTimeout); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("等待連線就緒失敗: %w", err)
		}

		// 緩存連接
		d.connsMu.Lock()
		if d.conns[serviceName] == nil {
			d.conns[serviceName] = make(map[string]*grpc.ClientConn)
		}
		d.conns[serviceName][instanceID] = conn
		d.connsMu.Unlock()

		// 同步到全局連接池
		pool := GetPool()
		pool.Add(serviceName, instanceID, conn)

		return conn, nil
	}

	// 沒有指定 instanceID，使用 Round Robin 選擇
	instances, err := d.GetInstances(serviceName, port)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, fmt.Errorf("服務 %s 沒有可用實例", serviceName)
	}

	// Round Robin 選擇
	d.rrMu.Lock()
	index := d.roundRobin[serviceName]
	d.roundRobin[serviceName] = (index + 1) % len(instances)
	selectedAddress := instances[index]
	d.rrMu.Unlock()

	// 建立連接
	conn, err := grpc.Dial(
		selectedAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("連接服務失敗: %w", err)
	}
	if err := waitForReady(conn, connectReadyTimeout); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("等待連線就緒失敗: %w", err)
	}

	// 緩存連接（使用地址作為 instanceID）
	d.connsMu.Lock()
	if d.conns[serviceName] == nil {
		d.conns[serviceName] = make(map[string]*grpc.ClientConn)
	}
	d.conns[serviceName][selectedAddress] = conn
	d.connsMu.Unlock()

	// 同步到全局連接池
	pool := GetPool()
	pool.Add(serviceName, selectedAddress, conn)

	return conn, nil
}

// GetConnection 獲取已建立的連接
func (d *EtcdDiscovery) GetConnection(serviceName, instanceID string) (*grpc.ClientConn, error) {
	d.connsMu.RLock()
	defer d.connsMu.RUnlock()

	if d.conns[serviceName] == nil {
		return nil, fmt.Errorf("服務 %s 沒有可用實例", serviceName)
	}

	// 如果指定了 instanceID，返回該實例的連接
	if instanceID != "" {
		if conn, exists := d.conns[serviceName][instanceID]; exists {
			state := conn.GetState()
			if state == connectivity.Ready || state == connectivity.Idle {
				return conn, nil
			}
			logger.Debugf("連接狀態異常: %s/%s, 狀態: %v", serviceName, instanceID, state)
		}
		return nil, fmt.Errorf("實例 %s/%s 連接不存在或狀態異常", serviceName, instanceID)
	}

	// 沒有指定 instanceID，返回第一個可用連接（僅 READY/IDLE，避免 CONNECTING 導致並發 RPC 失敗）
	for _, conn := range d.conns[serviceName] {
		state := conn.GetState()
		if state == connectivity.Ready || state == connectivity.Idle {
			return conn, nil
		}
	}

	return nil, fmt.Errorf("服務 %s 沒有可用實例", serviceName)
}

// getKeys 輔助函數：獲取 map 的所有 key
func getKeys(m map[string]*grpc.ClientConn) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Close 關閉所有連接和資源
func (d *EtcdDiscovery) Close() error {
	// 停止所有監聽
	d.watchersMu.Lock()
	for _, cancel := range d.watchers {
		cancel()
	}
	d.watchers = make(map[string]context.CancelFunc)
	d.watchersMu.Unlock()

	// 取消所有服務註冊
	d.leasesMu.Lock()
	for key, leaseID := range d.leases {
		parts := strings.Split(key, ":")
		if len(parts) == 2 {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, _ = d.etcdClient.Cli.Revoke(ctx, leaseID)
			cancel()
		}
	}
	d.leases = make(map[string]clientv3.LeaseID)
	d.leasesMu.Unlock()

	// 關閉所有連接
	d.connsMu.Lock()
	for _, serviceConns := range d.conns {
		for _, conn := range serviceConns {
			_ = conn.Close()
		}
	}
	d.conns = make(map[string]map[string]*grpc.ClientConn)
	d.connsMu.Unlock()

	return nil
}

// keepAlive 自動續約
func (d *EtcdDiscovery) keepAlive(serviceName, instanceID string, leaseID clientv3.LeaseID) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := d.etcdClient.Cli.KeepAlive(ctx, leaseID)
	if err != nil {
		logger.Errorf("啟動自動續約失敗: %v", err)
		return
	}

	for ka := range ch {
		if ka == nil {
			logger.Warnf("租約已過期: %s/%s", serviceName, instanceID)
			return
		}
	}
}

// updateConnections 更新連接池
func (d *EtcdDiscovery) updateConnections(serviceName string, instances []string) {
	d.connsMu.Lock()
	defer d.connsMu.Unlock()

	// 獲取全局連接池
	pool := GetPool()

	// 獲取當前連接
	serviceConns, exists := d.conns[serviceName]
	if !exists {
		serviceConns = make(map[string]*grpc.ClientConn)
		d.conns[serviceName] = serviceConns
	}

	// 創建實例地址集合
	instanceSet := make(map[string]bool)
	for _, instance := range instances {
		instanceSet[instance] = true
	}

	// 移除不存在的連接
	for address, conn := range serviceConns {
		if !instanceSet[address] {
			_ = conn.Close()
			delete(serviceConns, address)
			pool.Remove(serviceName, address)
		}
	}

	// 為新實例建立連接
	for _, address := range instances {
		if existingConn, exists := serviceConns[address]; exists {
			state := existingConn.GetState()
			if state != connectivity.TransientFailure && state != connectivity.Shutdown {
				continue
			}
			logger.Debugf("重新建立異常連接: %s/%s (狀態: %v)", serviceName, address, state)
			_ = existingConn.Close()
			delete(serviceConns, address)
			pool.Remove(serviceName, address)
		}
		conn, err := grpc.Dial(
			address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			logger.Debugf("建立連接失敗: %s/%s, %v", serviceName, address, err)
			continue
		}
		serviceConns[address] = conn
		pool.Add(serviceName, address, conn)
	}
}

// getServiceKey 獲取服務在 etcd 中的 key
func (d *EtcdDiscovery) getServiceKey(serviceName, instanceID string) string {
	return path.Join(etcdServicePrefix, serviceName, instanceID)
}

// getServiceKeyPrefix 獲取服務在 etcd 中的 key 前綴
func (d *EtcdDiscovery) getServiceKeyPrefix(serviceName string) string {
	return path.Join(etcdServicePrefix, serviceName) + "/"
}

// GetInstanceID 獲取實例 ID（從環境變量或自動生成）
func GetInstanceID(serviceName string) string {
	// 優先從環境變量獲取
	if instanceID := os.Getenv("SERVICE_ID"); instanceID != "" {
		return instanceID
	}
	if instanceID := os.Getenv("INSTANCE_ID"); instanceID != "" {
		return instanceID
	}
	// 兼容舊環境變量（已廢棄，保留以向後兼容）
	if podName := os.Getenv("POD_NAME"); podName != "" {
		return podName
	}

	// 自動生成（使用 profile 的 GenerateSID 邏輯）
	return generateSID(serviceName)
}

// generateSID 自動生成服務實例 ID（簡化版，不依賴 profile 包）
func generateSID(svt string) string {
	// 使用時間戳 + 隨機數生成短 ID
	timestamp := time.Now().Unix()
	random := timestamp % 1000000
	return fmt.Sprintf("%s-%d", svt, random)
}

