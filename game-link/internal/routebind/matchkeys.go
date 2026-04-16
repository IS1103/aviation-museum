// Package routebind: 重構方案 2.2 轉場控管 Key（pending_room、create_lock、cleanup_lock、current_batch、match:seq）
package routebind

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	PendingRoomTTLSec   = 120   // 2.2、11.4
	CreateLockTTLSec    = 60    // 11.4
	CleanupLockTTLSec   = 10    // 11.8
	SeqKeyExpireSec     = 86400
	CronIdleThresholdSec = 180  // 11.19：IDLETIME > 180s 執行清理
	CronLockTTLSec      = 60   // 11.20：CronJob 鎖
)

func keySeq(date string) string   { return "match:seq:" + date }
func keyPendingRoom(matchID string) string   { return "match:pending_room:" + matchID }
func keyCreateLock(matchID string) string    { return "match:create_lock:" + matchID }
func keyCleanupLock(matchID string) string   { return "match:cleanup_lock:" + matchID }
func keyCurrentBatch(svt string, minP, maxP int) string {
	return fmt.Sprintf("match:current_batch:%s:%d:%d", svt, minP, maxP)
}

// DateToday 回傳當日 YYYYMMDD（用於 match:seq:YYYYMMDD）
func DateToday() string {
	return time.Now().Format("20060102")
}

// NextMatchSeq 對 match:seq:YYYYMMDD 做 INCR 並設定 EXPIRE 86400（11.12），回傳新序號。
func NextMatchSeq() (int64, error) {
	date := DateToday()
	k := keySeq(date)
	v, err := redisDo("INCR", k)
	if err != nil {
		return 0, err
	}
	seq, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("INCR reply not int64: %T", v)
	}
	_, _ = redisDo("EXPIRE", k, strconv.Itoa(SeqKeyExpireSec))
	return seq, nil
}

// PendingRoomPush 將 uid 加入 match:pending_room:{matchID}（RPUSH），並設定 TTL 120s。
func PendingRoomPush(matchID string, uid uint32) error {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return fmt.Errorf("matchID is empty")
	}
	k := keyPendingRoom(matchID)
	uidStr := strconv.FormatUint(uint64(uid), 10)
	_, err := redisDo("RPUSH", k, uidStr)
	if err != nil {
		return err
	}
	_, _ = redisDo("EXPIRE", k, strconv.Itoa(PendingRoomTTLSec))
	return nil
}

// PendingRoomRange 回傳 match:pending_room:{matchID} 內所有 UID（字串形式）。
func PendingRoomRange(matchID string) ([]string, error) {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return nil, fmt.Errorf("matchID is empty")
	}
	k := keyPendingRoom(matchID)
	v, err := redisDo("LRANGE", k, "0", "-1")
	if err != nil {
		return nil, err
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("LRANGE reply not array: %T", v)
	}
	out := make([]string, 0, len(arr))
	for _, el := range arr {
		s, _ := el.(string)
		out = append(out, s)
	}
	return out, nil
}

// PendingRoomLRem 自 match:pending_room:{matchID} 移除指定 uid（count=0 表示移除所有符合值，11.14）。
func PendingRoomLRem(matchID string, uid uint32) (int, error) {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return 0, fmt.Errorf("matchID is empty")
	}
	k := keyPendingRoom(matchID)
	uidStr := strconv.FormatUint(uint64(uid), 10)
	v, err := redisDo("LREM", k, "0", uidStr)
	if err != nil {
		return 0, err
	}
	n, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("LREM reply not int64: %T", v)
	}
	return int(n), nil
}

// PendingRoomDel 刪除 match:pending_room:{matchID}（湊滿開房成功或清理時）。
func PendingRoomDel(matchID string) error {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return fmt.Errorf("matchID is empty")
	}
	return deleteRedisValue(keyPendingRoom(matchID))
}

// CreateLockTryAcquire 嘗試取得 match:create_lock:{matchID}（SET NX EX 60），成功回傳 true。
func CreateLockTryAcquire(matchID string) (bool, error) {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return false, fmt.Errorf("matchID is empty")
	}
	return redisSetNXEX(keyCreateLock(matchID), "1", CreateLockTTLSec)
}

// CleanupLockTryAcquire 嘗試取得 match:cleanup_lock:{matchID}（SET NX EX 10），僅搶到者執行 Rollback（5.1、11.8）。
func CleanupLockTryAcquire(matchID string) (bool, error) {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return false, fmt.Errorf("matchID is empty")
	}
	return redisSetNXEX(keyCleanupLock(matchID), "1", CleanupLockTTLSec)
}

// CurrentBatchGet 讀取 match:current_batch:{svt}:{minP}:{maxP} 的 match_id；無則回傳空字串。
func CurrentBatchGet(svt string, minP, maxP int) (string, error) {
	svt = strings.TrimSpace(svt)
	if svt == "" {
		return "", fmt.Errorf("svt is empty")
	}
	k := keyCurrentBatch(svt, minP, maxP)
	v, err := getRedisValue(k)
	if err != nil {
		return "", err
	}
	return v, nil
}

// CurrentBatchSet 寫入 match:current_batch:{svt}:{minP}:{maxP} = matchID（無 TTL，由業務覆寫）。
func CurrentBatchSet(svt string, minP, maxP int, matchID string) error {
	svt = strings.TrimSpace(svt)
	if svt == "" {
		return fmt.Errorf("svt is empty")
	}
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return fmt.Errorf("matchID is empty")
	}
	k := keyCurrentBatch(svt, minP, maxP)
	return setRedisValue(k, matchID)
}

// redisSetNXEX 執行 SET key value NX EX seconds；回傳 true 表示設定成功（搶到鎖）。
func redisSetNXEX(key, value string, seconds int) (bool, error) {
	v, err := redisDo("SET", key, value, "NX", "EX", strconv.Itoa(seconds))
	if err != nil {
		return false, err
	}
	return v != nil, nil
}

// KeyIdleTimeSeconds 回傳 key 的 OBJECT IDLETIME（秒），key 不存在回傳 0（11.19）。
func KeyIdleTimeSeconds(key string) (int64, error) {
	v, err := redisDo("OBJECT", "IDLETIME", key)
	if err != nil {
		return 0, err
	}
	n, _ := v.(int64)
	return n, nil
}

const cronCleanupLockKey = "match:cron_cleanup_lock"

// CronCleanupLockTryAcquire 搶 CronJob 鎖（SET NX EX 60），僅搶到者執行掃描（11.20）。
func CronCleanupLockTryAcquire() (bool, error) {
	return redisSetNXEX(cronCleanupLockKey, "1", CronLockTTLSec)
}

// PendingRoomKeysForCron 回傳所有 match:pending_room:* 的 key，供 CronJob 掃描。
func PendingRoomKeysForCron() ([]string, error) {
	return ScanKeys("match:pending_room:*")
}

// MatchIDFromPendingRoomKey 從 key "match:pending_room:m_20260215_1" 取出 match_id。
func MatchIDFromPendingRoomKey(key string) string {
	const prefix = "match:pending_room:"
	if strings.HasPrefix(key, prefix) {
		return key[len(prefix):]
	}
	return ""
}
