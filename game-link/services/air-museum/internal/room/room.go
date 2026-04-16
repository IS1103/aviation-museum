package room

import (
	"context"
	"errors"
	"sync"

	"air-museum/config"
	pb "air-museum/proto/pb"
	"internal/webcore/conn"

	"google.golang.org/protobuf/proto"
)

var errRoomFull = errors.New("room full")

// Room 單一房間，人數上限由 config.MaxPlayers 決定；直連用。
type Room struct {
	mu                 sync.RWMutex
	uids               []uint32
	byUid              map[uint32]struct{}
	hostUid            uint32
	currentPhase       pb.GamePhase           // 當前遊戲階段（供 IsPhasePlaying 等用）
	disconnectedInGame map[uint32]struct{}   // 遊戲中斷線的 uid，等遊戲結束後再 Remove
	connMgr            *conn.ConnectionManager
}

var theRoom = &Room{
	uids:               make([]uint32, 0, 16),
	byUid:              make(map[uint32]struct{}),
	disconnectedInGame: make(map[uint32]struct{}),
	connMgr:            conn.GetConnectionManager(),
}

// Get 取得單例房間
func Get() *Room {
	return theRoom
}

// Add 加入房間；滿員回錯
func (r *Room) Add(uid uint32) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byUid[uid]; ok {
		return nil // 已在房內
	}
	if len(r.uids) >= config.GetMaxPlayers() {
		return errRoomFull
	}
	r.byUid[uid] = struct{}{}
	r.uids = append(r.uids, uid)
	return nil
}

// Remove 離開房間
func (r *Room) Remove(uid uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byUid, uid)
	for i, u := range r.uids {
		if u == uid {
			r.uids = append(r.uids[:i], r.uids[i+1:]...)
			return
		}
	}
}

// Contains 是否在房內
func (r *Room) Contains(uid uint32) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.byUid[uid]
	return ok
}

// PlayerCount 當前人數
func (r *Room) PlayerCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.uids)
}

// UIDs 複製一份當前房內 uid 列表
func (r *Room) UIDs() []uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]uint32, len(r.uids))
	copy(out, r.uids)
	return out
}

// BroadcastToRoom 對房內所有人發送二進位封包（不含自己可傳 excludeUID 0 表示不排除）
func (r *Room) BroadcastToRoom(ctx context.Context, data []byte, excludeUID uint32) {
	uids := r.UIDs()
	for _, uid := range uids {
		if excludeUID != 0 && uid == excludeUID {
			continue
		}
		_ = r.connMgr.SendToUser(ctx, uid, data)
	}
}

// BroadcastPackToRoom 對房內所有人廣播 gate.Pack（proto.Marshal）
func (r *Room) BroadcastPackToRoom(ctx context.Context, pack proto.Message, excludeUID uint32) {
	if pack == nil {
		return
	}
	data, err := proto.Marshal(pack)
	if err != nil {
		return
	}
	r.BroadcastToRoom(ctx, data, excludeUID)
}

// SetHost 設定房內主機（投影端 uid）
func (r *Room) SetHost(uid uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hostUid = uid
}

// Host 取得房內主機 uid，0 表示尚未設定
func (r *Room) Host() uint32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.hostUid
}

// SetCurrentPhase 儲存當前遊戲階段（pushGameState 時更新）
func (r *Room) SetCurrentPhase(phase pb.GamePhase) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.currentPhase = phase
}

// IsPhasePlaying 目前是否為遊戲中（state == GAME_PHASE_PLAYING）
func (r *Room) IsPhasePlaying() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentPhase == pb.GamePhase_GAME_PHASE_PLAYING
}

// AddDisconnectedInGame 記錄「遊戲中斷線」的 uid，稍後遊戲結束時再 Remove
func (r *Room) AddDisconnectedInGame(uid uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.disconnectedInGame[uid] = struct{}{}
}

// ClearDisconnectedInGameAndReturn 回傳並清空「遊戲中斷線」名單，供遊戲結束時 Remove 用
func (r *Room) ClearDisconnectedInGameAndReturn() []uint32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]uint32, 0, len(r.disconnectedInGame))
	for u := range r.disconnectedInGame {
		out = append(out, u)
	}
	r.disconnectedInGame = make(map[uint32]struct{})
	return out
}
