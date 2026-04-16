package game

import (
	"sync"

	"internal.proto/pb/game"
)

var (
	// globalRoomManager 全局房間管理器實例（單例）
	globalRoomManager *RoomManager
	roomManagerOnce   sync.Once
)

// RoomManager 房間管理器
// 使用泛型和工廠模式，支援任何實現 Game 介面的遊戲類型
type RoomManager struct {
	rooms        map[string]Game   // Room ID -> Game 實例的映射（一個房間對應一個遊戲實例），rid 為 string
	playerToRoom map[uint32]string // 玩家 UID -> 房間 ID 映射
	roomMembers  map[string]map[uint32]struct{}
	mutex        sync.RWMutex // 讀寫鎖，保護並發訪問
}

// NewRoomManager 創建新的房間管理器
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms:        make(map[string]Game),
		playerToRoom: make(map[uint32]string),
		roomMembers:  make(map[string]map[uint32]struct{}),
	}
}

// CreateRoom 創建新房間
// rid: 房間 ID（string 類型）
// roomType: 房間類型
// password: 房間密碼
// gameFactory: 工廠函數，用於創建具體的遊戲實例
// 返回創建的遊戲實例
func (rm *RoomManager) CreateRoom(rid string, rType game.RoomType, password string, gameFactory func(string, game.RoomType, string) Game) Game {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	// 檢查房間是否已存在
	if _, exists := rm.rooms[rid]; exists {
		// 房間已存在，不創建新房間
		return nil
	}

	// 使用工廠函數創建遊戲實例（工廠函數內部會從 pool 獲取）
	game := gameFactory(rid, rType, password)

	// 將遊戲實例添加到管理器
	rm.rooms[rid] = game

	return game
}

// RemoveRoom 移除房間並將物件放回池中
func (rm *RoomManager) RemoveRoom(rid string) bool {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	game, exists := rm.rooms[rid]
	if !exists {
		return false
	}

	// 從管理器中移除房間
	delete(rm.rooms, rid)
	if members, ok := rm.roomMembers[rid]; ok {
		for uid := range members {
			delete(rm.playerToRoom, uid)
		}
		delete(rm.roomMembers, rid)
	}

	// 將物件放回池中
	game.Reset()
	return true
}

// GetRoom 獲取指定類型的房間實例（使用泛型，避免重複類型斷言）
// T: 目標遊戲類型
// rid: 房間 ID（string 類型）
// 返回具體類型的遊戲實例和是否存在
func GetRoom[T Game](rm *RoomManager, rid string) (T, bool) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	game, exists := rm.rooms[rid]
	if !exists {
		var zero T
		return zero, false
	}

	// 只進行一次類型斷言
	specificGame, ok := game.(T)
	return specificGame, ok
}

// GetGlobalRoomManager 獲取全局房間管理器實例（單例模式）
func GetGlobalRoomManager() *RoomManager {
	roomManagerOnce.Do(func() {
		globalRoomManager = NewRoomManager()
	})
	return globalRoomManager
}

// GetAllRooms 獲取所有房間的副本（用於查找操作）
// 返回房間 ID 到房間實例的映射副本
func (rm *RoomManager) GetAllRooms() map[string]Game {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	// 創建副本以避免持有鎖的時間過長
	roomsCopy := make(map[string]Game, len(rm.rooms))
	for rid, game := range rm.rooms {
		roomsCopy[rid] = game
	}
	return roomsCopy
}

// BindPlayer 將玩家與房間建立映射
func (rm *RoomManager) BindPlayer(rid string, uid uint32) {
	if uid == 0 {
		return
	}

	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if _, exists := rm.rooms[rid]; !exists {
		return
	}

	rm.playerToRoom[uid] = rid
	members, ok := rm.roomMembers[rid]
	if !ok {
		members = make(map[uint32]struct{})
		rm.roomMembers[rid] = members
	}
	members[uid] = struct{}{}
}

// UnbindPlayer 移除玩家與房間的映射
func (rm *RoomManager) UnbindPlayer(uid uint32) {
	if uid == 0 {
		return
	}

	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	rid, ok := rm.playerToRoom[uid]
	if !ok {
		return
	}

	delete(rm.playerToRoom, uid)
	if members, exists := rm.roomMembers[rid]; exists {
		delete(members, uid)
		if len(members) == 0 {
			delete(rm.roomMembers, rid)
		}
	}
}

// RidByUid 根據玩家 UID 查詢房間 ID
func (rm *RoomManager) RidByUid(uid uint32) (string, bool) {
	if uid == 0 {
		return "", false
	}

	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	rid, ok := rm.playerToRoom[uid]
	return rid, ok
}

// MembersByRoom 返回指定房間中的玩家列表
func (rm *RoomManager) MembersByRoom(rid string) []uint32 {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	members := rm.roomMembers[rid]
	if len(members) == 0 {
		return nil
	}

	result := make([]uint32, 0, len(members))
	for uid := range members {
		result = append(result, uid)
	}
	return result
}
