package handler

import (
	"air-museum/internal/room"
	"internal/webcore/conn"
)

// RegisterDisconnectCleanup 註冊斷線回調：玩家不在遊戲中則立即從房間移除；若在遊戲中則僅記錄，等遊戲結束後由 pushGameState 再移除。
func RegisterDisconnectCleanup() {
	conn.GetConnectionManager().RegisterDisconnectCallback(func(uid uint32) {
		r := room.Get()
		if !r.Contains(uid) {
			return
		}
		if r.IsPhasePlaying() {
			r.AddDisconnectedInGame(uid)
			return
		}
		r.Remove(uid)
	})
}
