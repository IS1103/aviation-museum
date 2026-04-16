package grpc

import (
	"fmt"
	"sync"

	"internal/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// ConnectionPool gRPC 連接池
type ConnectionPool struct {
	connections map[string]map[string]*grpc.ClientConn // serviceName -> instanceID -> conn
	mu          sync.RWMutex
	roundRobin  map[string]int // serviceName -> 當前索引（用於 round robin）
	rrMu        sync.Mutex
}

// NewConnectionPool 創建連接池
func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		connections: make(map[string]map[string]*grpc.ClientConn),
		roundRobin:  make(map[string]int),
	}
}

// Add 添加連接
func (p *ConnectionPool) Add(serviceName, instanceID string, conn *grpc.ClientConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.connections[serviceName] == nil {
		p.connections[serviceName] = make(map[string]*grpc.ClientConn)
	}

	// 如果已存在連接，先關閉舊連接
	if oldConn, exists := p.connections[serviceName][instanceID]; exists {
		_ = oldConn.Close()
	}

	p.connections[serviceName][instanceID] = conn
	logger.Debugf("連接已添加到池: %s/%s", serviceName, instanceID)
}

// Remove 移除連接
func (p *ConnectionPool) Remove(serviceName, instanceID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if serviceConns, exists := p.connections[serviceName]; exists {
		if conn, exists := serviceConns[instanceID]; exists {
			_ = conn.Close()
			delete(serviceConns, instanceID)
			logger.Debugf("連接已從池中移除: %s/%s", serviceName, instanceID)
		}

		// 如果該服務沒有實例了，清理 map
		if len(serviceConns) == 0 {
			delete(p.connections, serviceName)
		}
	}
}

// Pick 選擇一個連接（Round Robin）
func (p *ConnectionPool) Pick(serviceName string) (*grpc.ClientConn, error) {
	p.mu.RLock()
	serviceConns, exists := p.connections[serviceName]
	p.mu.RUnlock()

	if !exists || len(serviceConns) == 0 {
		return nil, fmt.Errorf("服務 %s 沒有可用連接", serviceName)
	}

	// 獲取所有實例 ID
	instanceIDs := make([]string, 0, len(serviceConns))
	for id := range serviceConns {
		instanceIDs = append(instanceIDs, id)
	}

	// Round Robin 選擇
	p.rrMu.Lock()
	index := p.roundRobin[serviceName]
	p.roundRobin[serviceName] = (index + 1) % len(instanceIDs)
	selectedID := instanceIDs[index]
	p.rrMu.Unlock()

	conn := serviceConns[selectedID]

	// 檢查連接狀態
	if conn.GetState() == connectivity.TransientFailure || conn.GetState() == connectivity.Shutdown {
		// 連接不可用，嘗試選擇下一個
		logger.Warnf("連接 %s/%s 狀態異常: %v，嘗試下一個", serviceName, selectedID, conn.GetState())
		return p.Pick(serviceName) // 遞歸選擇下一個
	}

	return conn, nil
}

// PickWithInstanceID 選擇一個連接並回傳其實例 ID（Round Robin），供 Match CAS 寫入 sid 用。
func (p *ConnectionPool) PickWithInstanceID(serviceName string) (*grpc.ClientConn, string, error) {
	p.mu.RLock()
	serviceConns, exists := p.connections[serviceName]
	p.mu.RUnlock()
	if !exists || len(serviceConns) == 0 {
		return nil, "", fmt.Errorf("服務 %s 沒有可用連接", serviceName)
	}
	instanceIDs := make([]string, 0, len(serviceConns))
	for id := range serviceConns {
		instanceIDs = append(instanceIDs, id)
	}
	p.rrMu.Lock()
	index := p.roundRobin[serviceName]
	p.roundRobin[serviceName] = (index + 1) % len(instanceIDs)
	selectedID := instanceIDs[index]
	p.rrMu.Unlock()
	conn := serviceConns[selectedID]
	if conn.GetState() == connectivity.TransientFailure || conn.GetState() == connectivity.Shutdown {
		return p.PickWithInstanceID(serviceName)
	}
	return conn, selectedID, nil
}

// GetConnection 獲取指定實例的連接
func (p *ConnectionPool) GetConnection(serviceName, instanceID string) (*grpc.ClientConn, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	serviceConns, exists := p.connections[serviceName]
	if !exists {
		return nil, fmt.Errorf("服務 %s 不存在", serviceName)
	}

	conn, exists := serviceConns[instanceID]
	if !exists {
		return nil, fmt.Errorf("實例 %s/%s 不存在", serviceName, instanceID)
	}

	return conn, nil
}

// Close 關閉所有連接
func (p *ConnectionPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for serviceName, serviceConns := range p.connections {
		for instanceID, conn := range serviceConns {
			_ = conn.Close()
			logger.Infof("關閉連接: %s/%s", serviceName, instanceID)
		}
		delete(p.connections, serviceName)
	}
}

// Dial 建立 gRPC 連接
func Dial(address string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), // 阻塞直到連接建立
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}
	return conn, nil
}

// DialNonBlocking 建立 gRPC 連接（非阻塞）
func DialNonBlocking(address string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// 不使用 WithBlock，允許異步連接
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}
	return conn, nil
}

