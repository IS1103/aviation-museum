package fixedroomhandlers

import (
	"context"
	"fmt"

	mgr "internal/game"
	"internal/routebind"

	gatepb "internal.proto/pb/gate"
)

// LeaveConfig 固定房間 Leave 流程所需配置
type LeaveConfig struct {
	Svt     string
	GetRID  func() string
	GetGame func(rm *mgr.RoomManager, rid string) (mgr.Game, bool)
}

// Leave 固定房間的 Leave 流程：只做邏輯（Leave、DelStatus），成功/失敗的推播由上層處理。
// 失敗時 return (nil, error)；成功時 return (info, nil)，上層負責 PushLeft。
func Leave(ctx context.Context, uid uint32, cfg LeaveConfig) (*gatepb.EntryAndLeaveInfo, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid empty")
	}

	rid := cfg.GetRID()
	if rid == "" {
		return nil, fmt.Errorf("rid not configured")
	}

	roomManager := mgr.GetGlobalRoomManager()
	game, exists := cfg.GetGame(roomManager, rid)
	if !exists {
		return nil, fmt.Errorf("room not found")
	}

	if err := game.Leave(uid); err != nil {
		return nil, fmt.Errorf("leave failed: %w", err)
	}
	_ = routebind.DelStatus(uid)
	return &gatepb.EntryAndLeaveInfo{Svt: cfg.Svt, Action: 2, Uid: uid, Rid: rid}, nil
}
