package matchhandlers

import (
	"context"
	"fmt"
	"log"
	"strconv"

	mgr "internal/game"
	"internal/playerdata"
	"internal/routebind"

	pbgame "internal.proto/pb/game"
	gatepb "internal.proto/pb/gate"
)

// AddPlayerToRoomConfig 補人流程所需配置
type AddPlayerToRoomConfig struct {
	Svt            string
	GetSvt         func() string
	GetGame        func(rm *mgr.RoomManager, rid string) (mgr.Game, bool)
	GetMinPlayer   func() int32
	GetPlayerCount func(game mgr.Game) int
	GetMaxPlayer   func() int
	CanAddPlayer              func(game mgr.Game) bool   // 是否允許補人（如 WAITING_TO_START）
	NotifyGameStateUpdate     func(game mgr.Game)       // 可選，補人後通知狀態更新
	AfterRegisterMatchableRoom func(game mgr.Game, minP int, ruleHash string) // 可選，註冊可湊桌後寫回遊戲（解散時 RemoveMatchableRoom 用）
}

// AddPlayerToRoom 湊桌補人：依 table_id 找到房間，加入 uids，寫 status、更新 matchable_rooms、推播。
func AddPlayerToRoom(ctx context.Context, req *pbgame.AddPlayerToRoomReq, cfg AddPlayerToRoomConfig) (*gatepb.Pack, error) {
	if req == nil || req.GetTableId() == "" || len(req.GetUids()) == 0 {
		return &gatepb.Pack{Success: false, Msg: "AddPlayerToRoomReq.table_id and uids required"}, nil
	}
	tableID := req.GetTableId()
	uids := req.GetUids()
	roomManager := mgr.GetGlobalRoomManager()
	game, ok := cfg.GetGame(roomManager, tableID)
	if !ok {
		return &gatepb.Pack{Success: false, Msg: fmt.Sprintf("room %s not found", tableID)}, nil
	}
	svt := cfg.GetSvt()
	if svt == "" {
		svt = cfg.Svt
	}
	minP := cfg.GetMinPlayer()
	if minP < 0 {
		minP = 0
	}
	maxP := cfg.GetMaxPlayer()
	if maxP <= 0 {
		maxP = 9
	}
	currentCount := cfg.GetPlayerCount(game)
	if currentCount >= maxP {
		return &gatepb.Pack{Success: false, Msg: "room full"}, nil
	}
	if cfg.CanAddPlayer != nil && !cfg.CanAddPlayer(game) {
		return &gatepb.Pack{Success: false, Msg: "room not accepting players"}, nil
	}
	ruleHash := req.GetRuleHash()
	if ruleHash == "" {
		return &gatepb.Pack{Success: false, Msg: "rule_hash is required"}, nil
	}

	entered := make([]uint32, 0, len(uids))
	for _, uid := range uids {
		if uid == 0 {
			continue
		}
		if currentCount+len(entered) >= maxP {
			break
		}
		uidStr := strconv.FormatUint(uint64(uid), 10)
		player, err := playerdata.Fetch(ctx, uidStr, 0)
		if err != nil {
			log.Printf("⚠️ AddPlayerToRoom Fetch uid=%d err=%v", uid, err)
			PushErrorToUids(ctx, []uint32{uid}, "取得玩家資料失敗")
			continue
		}
		player.Svt = svt
		if err := game.Entry(player); err != nil {
			log.Printf("⚠️ AddPlayerToRoom Entry rid=%s uid=%d err=%v", tableID, uid, err)
			PushErrorToUids(ctx, []uint32{uid}, err.Error())
			continue
		}
		roomManager.BindPlayer(tableID, uid)
		_ = routebind.SetStatusGamingWithTableID(uid, svt, tableID)
		entered = append(entered, uid)
	}
	if len(entered) == 0 {
		return &gatepb.Pack{Success: false, Msg: "no player added"}, nil
	}
	newCount := currentCount + len(entered)
	_ = RegisterMatchableRoom(ctx, svt, int(minP), ruleHash, tableID, newCount, maxP)
	if cfg.AfterRegisterMatchableRoom != nil {
		cfg.AfterRegisterMatchableRoom(game, int(minP), ruleHash)
	}
	if newCount >= maxP {
		_ = RemoveMatchableRoom(ctx, svt, int(minP), ruleHash, tableID)
	}
	PushEntered(ctx, svt, tableID, entered)
	if cfg.NotifyGameStateUpdate != nil {
		cfg.NotifyGameStateUpdate(game)
	}
	if newCount >= int(minP) {
		_ = game.StartGame()
	}
	log.Printf("✅ AddPlayerToRoom rid=%s added %d players", tableID, len(entered))
	return &gatepb.Pack{Success: true}, nil
}
