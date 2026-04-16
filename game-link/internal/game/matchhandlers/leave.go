package matchhandlers

import (
	"context"
	"fmt"
	"log"

	mgr "internal/game"
	"internal/routebind"
)

// LeaveConfig Leave 流程所需配置
type LeaveConfig struct {
	Svt        string
	GetGame    func(rm *mgr.RoomManager, rid string) (mgr.Game, bool)
	CanLeave   func(game mgr.Game) bool
	AfterLeave func(rm *mgr.RoomManager, game mgr.Game, rid string) // 可選（如 holdem 解散時 RemoveRoom）
}

// Leave 湊桌遊戲的 Leave 流程：只做邏輯，成功/失敗的推播由上層處理。
// 回傳 (rid, leftFromMatch, err)。leftFromMatch 為 true 時上層應呼叫 PushMatchLeft(uid)；否則上層呼叫 PushLeft(ctx, svt, uid, rid)。
func Leave(ctx context.Context, uid uint32, cfg LeaveConfig) (rid string, leftFromMatch bool, err error) {
	if uid == 0 {
		return "", false, fmt.Errorf("uid is required")
	}

	roomManager := mgr.GetGlobalRoomManager()
	rid, inRoom := roomManager.RidByUid(uid)

	if !inRoom {
		if LeaveQueue(ctx, uid, cfg.Svt) {
			log.Printf("✅ 玩家 %d 已離開湊桌 queue svt=%s", uid, cfg.Svt)
			return "", true, nil
		}
		return "", true, nil // 不在房也不在 queue，上層仍推 match/left
	}

	game, ok := cfg.GetGame(roomManager, rid)
	if !ok {
		return "", false, fmt.Errorf("room not found")
	}
	if !cfg.CanLeave(game) {
		return "", false, fmt.Errorf("僅能在等待開始階段或解散時離開，目前狀態不允許")
	}
	if err := game.Leave(uid); err != nil {
		return "", false, fmt.Errorf("leave: %w", err)
	}
	roomManager.UnbindPlayer(uid)
	_ = routebind.DelStatus(uid)
	if err := routebind.DeleteServiceSidByUID(uid, cfg.Svt); err != nil {
		log.Printf("⚠️ Leave DeleteServiceSidByUID failed uid=%d svt=%s: %v", uid, cfg.Svt, err)
	}
	log.Printf("✅ 玩家 %d 離開房間 rid=%s svt=%s", uid, rid, cfg.Svt)

	if cfg.AfterLeave != nil {
		cfg.AfterLeave(roomManager, game, rid)
	}
	return rid, false, nil
}
