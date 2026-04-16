package matchhandlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	gateforward "internal/gateforward"
	"internal/routebind"
	gatepb "internal.proto/pb/gate"
	pbgame "internal.proto/pb/game"

	"google.golang.org/protobuf/types/known/anypb"
)

// RuleToRuleHash 將 rule 字串轉為 ruleHash（SHA256 hex）；rule 為空時回傳空字串（呼叫方須視為錯誤、不入隊）。
func RuleToRuleHash(rule string) string {
	if rule == "" {
		return ""
	}
	h := sha256.Sum256([]byte(rule))
	return hex.EncodeToString(h[:])
}

// EntryCreateMu 保護 entry 與 createRoom 的原子性
var EntryCreateMu sync.Mutex

// PushEntered 推送 entered 通知給指定玩家
func PushEntered(ctx context.Context, svt, rid string, uids []uint32) {
	if len(uids) == 0 || rid == "" {
		return
	}
	for _, uid := range uids {
		entryResp := &gatepb.EntryAndLeaveInfo{Svt: svt, Action: 1, Uid: uid, Rid: rid}
		info, err := anypb.New(entryResp)
		if err != nil {
			log.Printf("[matchhandlers] PushEntered anypb.New failed uid=%d: %v", uid, err)
			continue
		}
		gateforward.CallGatePushToUids(ctx, []uint32{uid}, svt, "entered", info)
	}
	log.Printf("[matchhandlers] PushEntered: 推送給 %d 個玩家 rid=%s svt=%s", len(uids), rid, svt)
}

// PushLeft 推送 left 通知給指定玩家
func PushLeft(ctx context.Context, svt string, uid uint32, rid string) {
	PushLeftWithMsg(ctx, svt, uid, rid, "")
}

// PushLeftWithMsg 推送 left 通知給指定玩家，可帶 msg（如：餘額不足被剔除）
func PushLeftWithMsg(ctx context.Context, svt string, uid uint32, rid string, msg string) {
	info, err := anypb.New(&gatepb.EntryAndLeaveInfo{Svt: svt, Action: 2, Uid: uid, Rid: rid, Msg: msg})
	if err != nil {
		log.Printf("[matchhandlers] PushLeftWithMsg anypb.New failed: %v", err)
		return
	}
	gateforward.CallGatePushToUids(ctx, []uint32{uid}, svt, "left", info)
}

// PushErrorToUids 推送錯誤給指定玩家（委派 gateforward，湊桌流程與固定房共用）。
func PushErrorToUids(ctx context.Context, uids []uint32, msg string) {
	gateforward.PushErrorToUids(ctx, uids, msg)
}

// PushMatchLeft 推送 match/left（湊桌中離開）給指定玩家。
func PushMatchLeft(ctx context.Context, uid uint32) {
	info, _ := anypb.New(&gatepb.EntryAndLeaveInfo{Svt: "match", Action: 2, Uid: uid})
	gateforward.CallGatePushToUids(ctx, []uint32{uid}, "match", "left", info)
}

// LeaveQueue 湊桌中離開：ZREM Queue、DEL status。若 status 非 matching 回傳 false。
// 推播 match/left 由上層依 Leave 回傳之 leftFromMatch 處理。
func LeaveQueue(ctx context.Context, uid uint32, svt string) bool {
	st, err := routebind.GetStatus(uid)
	if err != nil || st == nil || st["state"] != "matching" || st["svt"] != svt {
		return false
	}
	minPStr := st["minP"]
	ruleHash := st["ruleHash"]
	if minPStr == "" || ruleHash == "" {
		return false
	}
	minP, _ := strconv.Atoi(minPStr)
	if minP <= 0 {
		return false
	}
	qk := routebind.QueueKey(svt, minP, ruleHash)
	_ = routebind.QueueZRem(qk, uid)
	_ = routebind.DelStatus(uid)
	return true
}

// EnterQueueWithScore 入隊並寫入 status（score 為入隊時間戳）。若已 matching 且同 queue 則冪等直接回傳。ruleHash 為空時回傳錯誤（client 未傳 rule 不得入隊）。
func EnterQueueWithScore(ctx context.Context, uid uint32, svt string, minP int, ruleHash string, score float64) error {
	if ruleHash == "" {
		return fmt.Errorf("ruleHash is required")
	}
	st, _ := routebind.GetStatus(uid)
	if st != nil && st["state"] == "matching" && st["svt"] == svt && st["ruleHash"] == ruleHash {
		return nil // 冪等
	}
	qk := routebind.QueueKey(svt, minP, ruleHash)
	if err := routebind.QueueZAdd(qk, uid, score); err != nil {
		return err
	}
	return routebind.SetStatusMatching(uid, svt, minP, ruleHash)
}

// RegisterMatchableRoom 呼叫 Match API 註冊或更新可湊桌房間。ruleHash 為空時回傳錯誤。
func RegisterMatchableRoom(ctx context.Context, svt string, minP int, ruleHash string, tableID string, currentCount, needCount int) error {
	if ruleHash == "" {
		return fmt.Errorf("ruleHash is required")
	}
	req := &pbgame.MatchableRoomRegister{
		Svt:          svt,
		MinP:         int32(minP),
		RuleHash:     ruleHash,
		TableId:      tableID,
		CurrentCount: int32(currentCount),
		NeedCount:    int32(needCount),
	}
	info, err := anypb.New(req)
	if err != nil {
		return err
	}
	_, err = gateforward.CallServiceRequestWithInfoTimeout(ctx, "match", "registerMatchableRoom", info, 5*time.Second)
	return err
}

// RemoveMatchableRoom 呼叫 Match API 移除可湊桌房間（滿員、開局或解散時）。ruleHash 為空時回傳錯誤。
func RemoveMatchableRoom(ctx context.Context, svt string, minP int, ruleHash string, tableID string) error {
	if ruleHash == "" {
		return fmt.Errorf("ruleHash is required")
	}
	req := &pbgame.MatchableRoomRemove{Svt: svt, MinP: int32(minP), RuleHash: ruleHash, TableId: tableID}
	info, err := anypb.New(req)
	if err != nil {
		return err
	}
	_, err = gateforward.CallServiceRequestWithInfoTimeout(ctx, "match", "removeMatchableRoom", info, 5*time.Second)
	return err
}
