package fixedroomhandlers

import (
	"context"
	"log"

	mgr "internal/game"
	gateforward "internal/gateforward"
	"internal/routebind"

	pbgame "internal.proto/pb/game"
	gatepb "internal.proto/pb/gate"
)

// EntryConfig 固定房間 Entry 流程所需配置
type EntryConfig struct {
	Svt         string
	GetRID      func() string
	GetGame     func(rm *mgr.RoomManager, rid string) (mgr.Game, bool)
	FetchPlayer func(ctx context.Context, uid uint32) (*pbgame.Player, error)
	GetSID      func() string
}

// Entry 固定房間的 Entry 流程：先查 {uid}/status，CASE 3 推 gate/error；否則進入固定 rid、寫 status=gaming+sid（match_id 空，11.10）
func Entry(ctx context.Context, uid uint32, cfg EntryConfig) (*gatepb.EntryAndLeaveInfo, error) {
	if uid == 0 {
		gateforward.PushErrorToUids(ctx, []uint32{uid}, "uid empty")
		return nil, nil
	}
	rid := cfg.GetRID()
	if rid == "" {
		gateforward.PushErrorToUids(ctx, []uint32{uid}, "rid not configured")
		return nil, nil
	}
	status, _ := routebind.GetStatus(uid)
	if status != nil && status["svt"] != cfg.Svt {
		gateforward.PushErrorToUids(ctx, []uint32{uid}, "請先離開當前遊戲再進入")
		return nil, nil
	}
	roomManager := mgr.GetGlobalRoomManager()
	game, exists := cfg.GetGame(roomManager, rid)
	if !exists {
		gateforward.PushErrorToUids(ctx, []uint32{uid}, "room not found")
		return nil, nil
	}
	if game.IsInGame(uid) {
		return &gatepb.EntryAndLeaveInfo{Svt: cfg.Svt, Action: 1, Uid: uid, Rid: rid}, nil
	}
	player, err := cfg.FetchPlayer(ctx, uid)
	if err != nil {
		gateforward.PushErrorToUids(ctx, []uint32{uid}, "fetch player data failed: "+err.Error())
		return nil, nil
	}
	if err := game.Entry(player); err != nil {
		gateforward.PushErrorToUids(ctx, []uint32{uid}, "entry failed: "+err.Error())
		return nil, nil
	}
	sid := cfg.GetSID()
	if sid == "" {
		sid = cfg.Svt + "-1"
	}
	if err := routebind.SetStatusGamingDirect(uid, cfg.Svt, sid); err != nil {
		log.Printf("⚠️ fixedroom Entry SetStatusGamingDirect failed uid=%d: %v", uid, err)
	}
	return &gatepb.EntryAndLeaveInfo{Svt: cfg.Svt, Action: 1, Uid: uid, Rid: rid}, nil
}
