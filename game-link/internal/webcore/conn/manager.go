package conn

import (
	"context"
	"fmt"
	"sync"
	"time"

	"internal/logger"

	"github.com/coder/websocket"
	"go.uber.org/zap"
)

// DisconnectCallback 斷線回調函數類型
type DisconnectCallback func(uid uint32)

// ConnectionManager 管理所有用戶的 WebSocket 連接
type ConnectionManager struct {
	mu                  sync.RWMutex
	connections         map[uint32]*websocket.Conn // uid -> connection
	channels            map[string]map[uint32]bool // channel -> uid -> bool (訂閱關係)
	disconnectCallbacks []DisconnectCallback       // 斷線回調函數列表
	callbackMutex       sync.RWMutex               // 回調函數的讀寫鎖
}

var connManager = &ConnectionManager{
	connections:         make(map[uint32]*websocket.Conn),
	channels:            make(map[string]map[uint32]bool),
	disconnectCallbacks: make([]DisconnectCallback, 0),
}

// GetConnectionManager 獲取全局連接管理器實例
func GetConnectionManager() *ConnectionManager {
	return connManager
}

// Register 註冊新連接，如果用戶已存在連接則直接覆蓋
func (cm *ConnectionManager) Register(uid uint32, conn *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 如果該用戶已有連接，記錄日誌
	if _, exists := cm.connections[uid]; exists {
		logger.Info("User duplicate login, replacing old connection",
			zap.Uint32("uid", uid),
		)
	}

	// 直接覆蓋或新增連接
	cm.connections[uid] = conn
}

// RegisterDisconnectCallback 註冊斷線回調函數
func (cm *ConnectionManager) RegisterDisconnectCallback(callback DisconnectCallback) {
	cm.callbackMutex.Lock()
	defer cm.callbackMutex.Unlock()

	cm.disconnectCallbacks = append(cm.disconnectCallbacks, callback)
}

// Unregister 移除用戶連接
// 注意：此方法只會移除連接，不會移除頻道訂閱
// 頻道訂閱應該由業務邏輯層（如遊戲房間的 Leave 方法）來管理
// 這樣即使玩家斷線，頻道訂閱也會保留，重連後可以繼續接收消息
func (cm *ConnectionManager) Unregister(uid uint32) {
	cm.mu.Lock()
	wasConnected := false
		if _, exists := cm.connections[uid]; exists {
			delete(cm.connections, uid)
			wasConnected = true
		logger.GateInfo(fmt.Sprintf("[%d] disconnected, online: %d", uid, len(cm.connections)))
		}
	cm.mu.Unlock()

	// 如果連接存在，觸發斷線回調
	// 注意：不自動清理頻道訂閱，讓業務邏輯層決定何時取消訂閱
	// 這樣即使玩家斷線，頻道訂閱也會保留，重連後可以繼續接收消息
	if wasConnected {
		cm.triggerDisconnectCallbacks(uid)
	}
}

// triggerDisconnectCallbacks 觸發所有註冊的斷線回調函數
func (cm *ConnectionManager) triggerDisconnectCallbacks(uid uint32) {
	cm.callbackMutex.RLock()
	callbacks := make([]DisconnectCallback, len(cm.disconnectCallbacks))
	copy(callbacks, cm.disconnectCallbacks)
	cm.callbackMutex.RUnlock()

	// 在鎖外執行回調，避免阻塞
	for _, callback := range callbacks {
		callback(uid)
	}
}

// GetConnection 獲取指定用戶的連接
func (cm *ConnectionManager) GetConnection(uid uint32) (*websocket.Conn, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	conn, ok := cm.connections[uid]
	return conn, ok
}

// IsOnline 檢查用戶是否在線
func (cm *ConnectionManager) IsOnline(uid uint32) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, ok := cm.connections[uid]
	return ok
}

// GetOnlineCount 獲取當前在線用戶數
func (cm *ConnectionManager) GetOnlineCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.connections)
}

// GetAllOnlineUsers 獲取所有在線用戶 ID 列表
func (cm *ConnectionManager) GetAllOnlineUsers() []uint32 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	users := make([]uint32, 0, len(cm.connections))
	for uid := range cm.connections {
		users = append(users, uid)
	}
	return users
}

// Broadcast 向所有在線用戶廣播消息
// 如果某些玩家斷線或發送失敗，會記錄錯誤並優雅地跳過，不會移除玩家
func (cm *ConnectionManager) Broadcast(ctx context.Context, data []byte) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	successCount := 0
	failCount := 0
	for _, conn := range cm.connections {
		if err := conn.Write(ctx, websocket.MessageBinary, data); err != nil {
			// 發送失敗時只記錄錯誤，不移除玩家，優雅地跳過
			//
			// 注意：這是正常情況，因為：
			// 1. 重複登入時，舊連接會被關閉，發送消息失敗是預期的
			// 2. 玩家斷線但尚未離開房間時，連接已關閉但玩家數據需要保留（用於重連）
			// 所以這裡不記錄警告，靜默處理即可
			// logger.Warn("Failed to broadcast message to user (session disconnected or send failed), skipping gracefully",
			// 	zap.String("uid", uid),
			// 	zap.Error(err),
			// )
			failCount++
		} else {
			successCount++
		}
	}

	logger.Info("Broadcast completed",
		zap.Int("success", successCount),
		zap.Int("failed", failCount),
		zap.Int("total", len(cm.connections)),
	)
}

// SendToUser 向指定用戶發送消息
// 如果用戶斷線或發送失敗，會記錄錯誤並返回錯誤，但不會移除玩家
// 調用者應該優雅地處理錯誤，不要因為發送失敗而移除玩家或取消訂閱
func (cm *ConnectionManager) SendToUser(ctx context.Context, uid uint32, data []byte) error {
	conn, ok := cm.GetConnection(uid)
	if !ok {
		logger.Debug("User not online, cannot send message",
			zap.Uint32("uid", uid),
		)
		// 用戶不在線時返回 nil（不視為錯誤），優雅地跳過
		return nil
	}

	if err := conn.Write(ctx, websocket.MessageBinary, data); err != nil {
		// 發送失敗時記錄錯誤並返回錯誤，但不會自動移除玩家
		// 調用者應該優雅地處理這個錯誤，不要因為發送失敗而移除玩家或取消訂閱
		//
		// 注意：這是正常情況，因為：
		// 1. 重複登入時，舊連接會被關閉，發送消息失敗是預期的
		// 2. 玩家斷線但尚未離開房間時，連接已關閉但玩家數據需要保留（用於重連）
		// 所以這裡不記錄警告，靜默處理即可
		// logger.Warn("Failed to send message to user (session disconnected or send failed), skipping gracefully",
		// 	zap.String("uid", uid),
		// 	zap.Error(err),
		// )
		return err
	}

	return nil
}

// SubscribeChannel 訂閱頻道
func (cm *ConnectionManager) SubscribeChannel(channel string, uid uint32) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.channels[channel] == nil {
		cm.channels[channel] = make(map[uint32]bool)
	}
	cm.channels[channel][uid] = true

	logger.Info("User subscribed to channel",
		zap.Uint32("uid", uid),
		zap.String("channel", channel),
		zap.Int("subscribers", len(cm.channels[channel])),
	)
}

// UnsubscribeChannel 取消訂閱頻道
func (cm *ConnectionManager) UnsubscribeChannel(channel string, uid uint32) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if channelMap, exists := cm.channels[channel]; exists {
		delete(channelMap, uid)
		if len(channelMap) == 0 {
			delete(cm.channels, channel)
		}
		logger.Info("User unsubscribed from channel",
			zap.Uint32("uid", uid),
			zap.String("channel", channel),
			zap.Int("remainingSubscribers", len(channelMap)),
		)
	}
}

// UnsubscribeAllChannels 取消用戶的所有頻道訂閱（用於斷線時清理）
func (cm *ConnectionManager) UnsubscribeAllChannels(uid uint32) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for channel, channelMap := range cm.channels {
		if channelMap[uid] {
			delete(channelMap, uid)
			if len(channelMap) == 0 {
				delete(cm.channels, channel)
			}
			logger.Debug("User unsubscribed from channel (cleanup)",
				zap.Uint32("uid", uid),
				zap.String("channel", channel),
			)
		}
	}
}

// GetChannelSubscribers 獲取頻道的所有訂閱者 UID 列表
func (cm *ConnectionManager) GetChannelSubscribers(channel string) []uint32 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	channelMap, exists := cm.channels[channel]
	if !exists {
		return []uint32{}
	}

	subscribers := make([]uint32, 0, len(channelMap))
	for uid := range channelMap {
		subscribers = append(subscribers, uid)
	}
	return subscribers
}

// SendToChannel 向頻道的所有訂閱者發送消息
// 如果某些玩家斷線或發送失敗，會記錄錯誤並優雅地跳過，不會移除玩家或取消訂閱
func (cm *ConnectionManager) SendToChannel(ctx context.Context, channel string, data []byte) int {
	subscribers := cm.GetChannelSubscribers(channel)
	if len(subscribers) == 0 {
		return 0
	}

	successCount := 0
	failCount := 0
	for _, uid := range subscribers {
		// 嘗試發送消息，如果失敗則優雅地跳過，不移除玩家或取消訂閱
		if err := cm.SendToUser(ctx, uid, data); err == nil {
			successCount++
		} else {
			// 發送失敗時只記錄錯誤，不移除玩家或取消訂閱
			failCount++
			logger.Debug("Failed to send channel message to user, skipping gracefully",
				zap.Uint32("uid", uid),
				zap.String("channel", channel),
			)
		}
	}

	logger.Info("Message sent to channel",
		zap.String("channel", channel),
		zap.Int("success", successCount),
		zap.Int("failed", failCount),
		zap.Int("total", len(subscribers)),
	)

	return successCount
}

// HandleDuplicateLogin 處理重複登入
// 如果用戶已有舊連接，則向舊連接發送錯誤推送並關閉舊連接
// errorPackData: 錯誤推送的序列化數據（可選，如果為 nil 則只關閉連接不發送錯誤）
// 返回是否有舊連接被關閉
func (cm *ConnectionManager) HandleDuplicateLogin(ctx context.Context, uid uint32, errorPackData []byte) bool {
	cm.mu.Lock()
	oldConn, hasOldConn := cm.connections[uid]
	cm.mu.Unlock()

	if !hasOldConn || oldConn == nil {
		return false
	}

	logger.Info("Handling duplicate login",
		zap.Uint32("uid", uid),
	)

	// 向舊連接發送錯誤推送（如果提供了錯誤數據）
	if len(errorPackData) > 0 {
		logger.Info("Sending duplicate login error to old connection",
			zap.Uint32("uid", uid),
			zap.Int("dataSize", len(errorPackData)),
		)
		if err := oldConn.Write(ctx, websocket.MessageBinary, errorPackData); err != nil {
			logger.Warn("Failed to send duplicate login error to old connection",
				zap.Uint32("uid", uid),
				zap.Error(err),
			)
		} else {
			logger.Info("Duplicate login error sent successfully, waiting for client to receive",
				zap.Uint32("uid", uid),
			)
			// 等待足夠的時間，確保錯誤推送已發送並被客戶端接收
			// 注意：coder/websocket 的 Write 是同步的，會阻塞直到數據寫入完成
			// 但我們需要給客戶端一些時間來處理消息，然後再關閉連接
			time.Sleep(200 * time.Millisecond)
		}
	}

	// 關閉舊連接
	oldConn.Close(websocket.StatusNormalClosure, "Duplicate login detected")

	// 從 ConnectionManager 移除舊連接（不觸發斷線回調，因為這是主動關閉）
	cm.mu.Lock()
	if _, exists := cm.connections[uid]; exists && cm.connections[uid] == oldConn {
		delete(cm.connections, uid)
		logger.Info("Old connection removed due to duplicate login",
			zap.Uint32("uid", uid),
			zap.Int("onlineCount", len(cm.connections)),
		)
	}
	cm.mu.Unlock()

	return true
}

// CloseUserWithNotify 向指定用戶發送封包後關閉連線並自 map 移除（不觸發斷線回調）。
// 用於主機重連時踢掉房內玩家等情境。packData 可為 nil 表示只關閉不送。
func (cm *ConnectionManager) CloseUserWithNotify(ctx context.Context, uid uint32, packData []byte) bool {
	cm.mu.Lock()
	oldConn, hasOldConn := cm.connections[uid]
	cm.mu.Unlock()

	if !hasOldConn || oldConn == nil {
		return false
	}
	if len(packData) > 0 {
		_ = oldConn.Write(ctx, websocket.MessageBinary, packData)
		time.Sleep(100 * time.Millisecond)
	}
	oldConn.Close(websocket.StatusNormalClosure, "closed by server")
	cm.mu.Lock()
	if _, exists := cm.connections[uid]; exists && cm.connections[uid] == oldConn {
		delete(cm.connections, uid)
	}
	cm.mu.Unlock()
	return true
}
