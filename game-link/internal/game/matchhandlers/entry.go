package matchhandlers

import (
	"context"
	"log"
	"strconv"
	"time"

	mgr "internal/game"
	gateforward "internal/gateforward"
	"internal/playerdata"

	pbgame "internal.proto/pb/game"

	"google.golang.org/protobuf/types/known/anypb"
)

// EntryConfig Entry 流程所需配置
type EntryConfig struct {
	Svt       string
	MinPlayer uint32
	MaxPlayer uint32
	// MinBet 進入遊戲最低所需資產（0 表示不檢查）。德州撲克為大盲注，百家樂為最小下注。
	MinBet int32
	// MinBetReason 資產不足時的錯誤說明（可選，預設為「玩家資產不足，無法進入遊戲」）
	MinBetReason string
	// CoinType 依 entry rule 的幣別取得錢包，作為資產檢查與遊玩時扣款依據（0 為預設）
	CoinType uint32
}

// Entry 湊桌遊戲的 Entry 流程：已在房則 push entered；在 match 則略過；否則取得完整玩家資料（profile+wallet）後 Notify match/entry
func Entry(ctx context.Context, uid uint32, cfg EntryConfig) error {
	if uid == 0 {
		PushErrorToUids(ctx, []uint32{uid}, "uid is required")
		return nil
	}

	roomManager := mgr.GetGlobalRoomManager()
	if rid, ok := roomManager.RidByUid(uid); ok {
		log.Printf("[matchhandlers] Entry uid=%d 已在房間 rid=%s，推送 entered", uid, rid)
		PushEntered(ctx, cfg.Svt, rid, []uint32{uid})
		return nil
	}
	// 是否已在 Match 由 Match 端 byUid 判斷；路由已依 status.state==matching 轉到 Match

	minP, maxP := cfg.MinPlayer, cfg.MaxPlayer
	if minP == 0 {
		minP = 2
	}
	if maxP == 0 {
		maxP = 6
	}

	// 使用 playerdata.Fetch 取得完整玩家資料（含 profile + wallet），coinType 依 EntryConfig（通常來自 entry rule）
	ctxTimeout, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	uidStr := strconv.FormatUint(uint64(uid), 10)
	player, err := playerdata.Fetch(ctxTimeout, uidStr, cfg.CoinType)
	if err != nil {
		log.Printf("[matchhandlers] Entry uid=%d 取得玩家資料失敗: %v", uid, err)
		PushErrorToUids(ctx, []uint32{uid}, "取得玩家資料失敗: "+err.Error())
		return nil
	}

	// 檢查玩家資產是否足夠進入遊戲
	if cfg.MinBet > 0 {
		coin := player.GetCoin()
		if coin < 0 || int32(coin) < cfg.MinBet {
			reason := cfg.MinBetReason
			if reason == "" {
				reason = "玩家資產不足，無法進入遊戲"
			}
			log.Printf("[matchhandlers] Entry uid=%d 資產不足: coin=%d minBet=%d", uid, coin, cfg.MinBet)
			PushErrorToUids(ctx, []uint32{uid}, reason)
			return nil
		}
	}

	log.Printf("[matchhandlers] Entry uid=%d 加入配對 svt=%s minP=%d maxP=%d", uid, cfg.Svt, minP, maxP)
	info, _ := anypb.New(&pbgame.MatchEntry{
		Svt:       cfg.Svt,
		MinPlayer: minP,
		MaxPlayer: maxP,
		Player:    player,
	})
	gateforward.CallNotifyWithUID(ctx, uid, "match", "entry", info)
	return nil
}
