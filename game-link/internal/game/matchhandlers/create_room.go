package matchhandlers

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	mgr "internal/game"
	"internal/routebind"

	pbgame "internal.proto/pb/game"
	gatepb "internal.proto/pb/gate"
)

// CreateRoomConfig CreateRoom 流程所需配置
type CreateRoomConfig struct {
	Svt              string
	GetSvt           func() string
	GetSID           func() string
	RoomExists       func(rm *mgr.RoomManager, rid string) bool
	NewGame          func(rid string, roomType pbgame.RoomType, password string) mgr.Game
	GetMinPlayer     func() int32
	GetPlayerCount   func(game mgr.Game) int
	AfterRoomCreated func(game mgr.Game) // 可選，建立房間後執行（如 holdem 設定 BetTime）
	// AfterStartGame 可選，StartGame 後執行；傳入 req 以便取得 rule_hash 註冊 matchable_rooms
	AfterStartGame func(ctx context.Context, game mgr.Game, req *pbgame.MatchRoom)
}

// CreateRoom 湊桌遊戲的 CreateRoom 流程：依 MatchRoom 建立房間、入房、綁定、啟動遊戲、推播。
// 重要：create room 完成後，遊戲狀態一律為 WAITING_TO_START（由 NewGame 與 StartGame 保證）。
func CreateRoom(ctx context.Context, uid uint32, req *pbgame.MatchRoom, cfg CreateRoomConfig) (*gatepb.Pack, error) {
	EntryCreateMu.Lock()
	defer EntryCreateMu.Unlock()

	if req == nil {
		return &gatepb.Pack{Success: false, Msg: "MatchRoom is required"}, nil
	}
	rid := req.GetRid()
	svt := req.GetSvt()
	if rid == "" {
		// plan-simplify-match-dispatch §3.4：由 Game 產生 table_id = {svt}-{sid}-{time}-{players_uid}
		players := req.GetPlayers()
		uids := make([]uint32, 0, len(players))
		for _, p := range players {
			if p != nil && p.GetUid() != 0 {
				uids = append(uids, p.GetUid())
			}
		}
		sort.Slice(uids, func(i, j int) bool { return uids[i] < uids[j] })
		parts := make([]string, len(uids))
		for i, uid := range uids {
			parts[i] = strconv.FormatUint(uint64(uid), 10)
		}
		rid = fmt.Sprintf("%s-%s-%s-%s", svt, cfg.GetSID(), time.Now().Format("20060102150405"), strings.Join(parts, "_"))
	}
	cfgSvt := cfg.GetSvt()
	if svt != "" && svt != cfgSvt {
		return &gatepb.Pack{Success: false, Msg: fmt.Sprintf("MatchRoom.svt %s does not match this service", svt)}, nil
	}
	players := req.GetPlayers()
	if len(players) == 0 {
		return &gatepb.Pack{Success: false, Msg: "MatchRoom.players is required"}, nil
	}

	roomManager := mgr.GetGlobalRoomManager()
	if cfg.RoomExists(roomManager, rid) {
		return &gatepb.Pack{Success: false, Msg: fmt.Sprintf("room %s already exists", rid)}, nil
	}

	created := roomManager.CreateRoom(rid, pbgame.RoomType_PUBLIC, "", func(roomID string, roomType pbgame.RoomType, password string) mgr.Game {
		return cfg.NewGame(roomID, roomType, password)
	})
	if created == nil {
		return &gatepb.Pack{Success: false, Msg: fmt.Sprintf("room %s create failed", rid)}, nil
	}

	if cfg.AfterRoomCreated != nil {
		cfg.AfterRoomCreated(created)
	}

	enteredUIDs := make([]uint32, 0, len(players))
	roomSvt := req.GetSvt()
	if roomSvt == "" {
		roomSvt = cfgSvt
	}
	for _, p := range players {
		if p == nil || p.GetUid() == 0 {
			continue
		}
		if p.GetSvt() == "" && roomSvt != "" {
			p.Svt = roomSvt
		}
		if err := created.Entry(p); err != nil {
			log.Printf("⚠️ CreateRoom entry failed rid=%s uid=%d: %v", rid, p.GetUid(), err)
			PushErrorToUids(ctx, []uint32{p.GetUid()}, err.Error())
			continue
		}
		roomManager.BindPlayer(rid, p.GetUid())
		enteredUIDs = append(enteredUIDs, p.GetUid())
	}
	for _, uid := range enteredUIDs {
		_ = routebind.SetStatusGamingWithTableID(uid, roomSvt, rid)
	}

	// 重要：先推播 entered，再 StartGame。否則 StartGame 的 state machine 會立即發送 gameStateUpdate，
	// 若客戶端在 LoadingScene 先收到 gameStateUpdate（未註冊）而 entered 尚未送達，會导致一直等待 entered。
	if len(enteredUIDs) > 0 {
		PushEntered(ctx, cfgSvt, rid, enteredUIDs)
	}

	minPlayer := cfg.GetMinPlayer()
	if minPlayer < 0 {
		minPlayer = 0
	}
	if cfg.GetPlayerCount(created) >= int(minPlayer) {
		if err := created.StartGame(); err != nil {
			log.Printf("⚠️ CreateRoom StartGame failed rid=%s: %v", rid, err)
		} else if cfg.AfterStartGame != nil {
			cfg.AfterStartGame(ctx, created, req)
		}
	}

	log.Printf("✅ CreateRoom from match rid=%s players=%d svt=%s", rid, len(players), cfgSvt)
	return &gatepb.Pack{Success: true}, nil
}
