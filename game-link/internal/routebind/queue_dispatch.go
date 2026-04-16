// Package routebind: plan-simplify-match-dispatch 用 Queue ZSET、matchable_rooms Hash、Timer 鎖與 status 輔助。
package routebind

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// TimerLockTTLSec 建議略大於 Timer 週期（如 1.5s）；拿不到鎖即跳過本輪。
	TimerLockTTLSec = 2
)

// --- Queue ZSET: Queue:{svt}:{minP}:{ruleHash} ---

// QueueKey 回傳 Queue ZSET 的 Redis key。
func QueueKey(svt string, minP int, ruleHash string) string {
	return fmt.Sprintf("Queue:%s:%d:%s", svt, minP, ruleHash)
}

// QueueZAdd 將 uid 加入 queue，score 為入隊時間戳（用於排序）。
func QueueZAdd(queueKey string, uid uint32, score float64) error {
	uidStr := strconv.FormatUint(uint64(uid), 10)
	scoreStr := strconv.FormatFloat(score, 'f', -1, 64)
	_, err := redisDo("ZADD", queueKey, scoreStr, uidStr)
	return err
}

// QueueZRem 自 queue 移除 uid。
func QueueZRem(queueKey string, uid uint32) error {
	uidStr := strconv.FormatUint(uint64(uid), 10)
	_, err := redisDo("ZREM", queueKey, uidStr)
	return err
}

// QueueZCard 回傳 queue 內成員數量。
func QueueZCard(queueKey string) (int, error) {
	v, err := redisDo("ZCARD", queueKey)
	if err != nil {
		return 0, err
	}
	n, _ := v.(int64)
	return int(n), nil
}

// luaQueueZRangeAndRem 原子：自 ZSET 取 score 最小的 n 個 member，並從 ZSET 中移除。
// KEYS[1] = queue key, ARGV[1] = n (number to take)
// 回傳取到的 uid 字串陣列（可能少於 n 若 queue 不足）。
const luaQueueZRangeAndRem = `
local n = tonumber(ARGV[1])
if n <= 0 then return {} end
local uids = redis.call("ZRANGE", KEYS[1], 0, n - 1)
if #uids > 0 then
  redis.call("ZREM", KEYS[1], unpack(uids))
end
return uids
`

// QueueZRangeAndRem 原子取前 n 個 uid 並自 queue 移除；回傳取到的 uid 字串 slice。
func QueueZRangeAndRem(queueKey string, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	reply, err := redisEval(luaQueueZRangeAndRem, []string{queueKey}, []string{strconv.Itoa(n)})
	if err != nil {
		return nil, err
	}
	arr, ok := reply.([]interface{})
	if !ok {
		return nil, fmt.Errorf("QueueZRangeAndRem reply not array: %T", reply)
	}
	out := make([]string, 0, len(arr))
	for _, el := range arr {
		s, _ := el.(string)
		if s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

// ParseQueueKey 從 "Queue:{svt}:{minP}:{ruleHash}" 解析出 svt, minP, ruleHash；無法解析時 ok=false。
func ParseQueueKey(key string) (svt string, minP int, ruleHash string, ok bool) {
	const prefix = "Queue:"
	if !strings.HasPrefix(key, prefix) {
		return "", 0, "", false
	}
	rest := key[len(prefix):]
	parts := strings.SplitN(rest, ":", 3)
	if len(parts) != 3 {
		return "", 0, "", false
	}
	svt = parts[0]
	var err error
	minP, err = strconv.Atoi(parts[1])
	if err != nil || minP <= 0 {
		return "", 0, "", false
	}
	ruleHash = parts[2]
	if ruleHash == "" {
		return "", 0, "", false
	}
	return svt, minP, ruleHash, true
}

// ScanQueueKeys 回傳所有 Redis key 符合 "Queue:*" 者（不篩 ZCARD，呼叫方自行過濾）。
func ScanQueueKeys() ([]string, error) {
	return ScanKeys("Queue:*")
}

// --- matchable_rooms Hash: matchable_rooms:{svt}:{minP}:{ruleHash} ---

// MatchableRoomsKey 回傳 matchable_rooms Hash 的 Redis key。
func MatchableRoomsKey(svt string, minP int, ruleHash string) string {
	return fmt.Sprintf("matchable_rooms:%s:%d:%s", svt, minP, ruleHash)
}

// MatchableRoomsHSet 設定一筆 table_id -> "current:need"；value 格式為 current_count:need_count。
func MatchableRoomsHSet(roomsKey, tableID string, currentCount, needCount int) error {
	val := fmt.Sprintf("%d:%d", currentCount, needCount)
	_, err := redisDo("HSET", roomsKey, tableID, val)
	return err
}

// MatchableRoomsHDel 自 matchable_rooms 移除一筆 table_id。
func MatchableRoomsHDel(roomsKey, tableID string) error {
	_, err := redisDo("HDEL", roomsKey, tableID)
	return err
}

// MatchableRoomsHGetAll 回傳 Hash 所有 field->value；value 為 "current:need"。
func MatchableRoomsHGetAll(roomsKey string) (map[string]string, error) {
	v, err := redisDo("HGETALL", roomsKey)
	if err != nil {
		return nil, err
	}
	arr, ok := v.([]interface{})
	if !ok || len(arr) == 0 {
		return map[string]string{}, nil
	}
	out := make(map[string]string)
	for i := 0; i+1 < len(arr); i += 2 {
		k, _ := arr[i].(string)
		val, _ := arr[i+1].(string)
		out[k] = val
	}
	return out, nil
}

// --- Timer 鎖: match:timer:{svt}:{minP}:{ruleHash} ---

// TimerLockKey 回傳 per-queue Timer 鎖的 Redis key。
func TimerLockKey(svt string, minP int, ruleHash string) string {
	return fmt.Sprintf("match:timer:%s:%d:%s", svt, minP, ruleHash)
}

// TimerLockTryAcquire 嘗試取得鎖（SET NX EX）；成功回傳 true，拿不到不重試（跳過本輪）。
func TimerLockTryAcquire(lockKey string, ttlSec int) (bool, error) {
	if ttlSec <= 0 {
		ttlSec = TimerLockTTLSec
	}
	return redisSetNXEX(lockKey, "1", ttlSec)
}

// --- status 輔助（Game 寫入 matching/gaming；Match 讀取過濾）---

// SetStatusMatching 寫入 {uid}/status 為 state=matching, svt, minP, ruleHash（Game 入隊時呼叫）。
func SetStatusMatching(uid uint32, svt string, minP int, ruleHash string) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	key := keyStatus(uid)
	minPStr := strconv.Itoa(minP)
	_, err := redisDo("HSET", key, "state", "matching", "svt", svt, "minP", minPStr, "ruleHash", ruleHash)
	return err
}

// SetStatusGamingWithTableID 寫入 {uid}/status 為 state=gaming, svt, table_id（Game 在 CreateRoom/AddPlayerToRoom 成功後呼叫）。
func SetStatusGamingWithTableID(uid uint32, svt, tableID string) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	key := keyStatus(uid)
	_, err := redisDo("HSET", key, "state", "gaming", "svt", svt, "table_id", tableID)
	return err
}

// FilterUidsStillMatching 回傳 uids 中仍為 state=matching 者（用於失敗還原時只把仍 matching 的放回 Queue）。
func FilterUidsStillMatching(uids []uint32) ([]uint32, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	// 單次 Lua：對多個 uid 檢查 state，回傳仍為 matching 的 uid 列表。
	// KEYS[i] = uid/status for uids[i], 但 Lua 不能動態 KEYS 數量。改為用 ARGV 傳 uid 列表，在 Lua 裡對每個 uid 做 HGET uid.."/status" "state"。
	// 實際上 KEYS 必須是 key 列表。所以 keys = [ "1/status", "2/status", ... ], ARGV 可不用。但 key 數量可能很大。Redis EVAL 的 KEYS 數量有限制，通常夠用。
	keys := make([]string, len(uids))
	for i, uid := range uids {
		keys[i] = keyStatus(uid)
	}
	reply, err := redisEval(luaFilterStillMatching, keys, nil)
	if err != nil {
		return nil, err
	}
	arr, ok := reply.([]interface{})
	if !ok {
		return nil, fmt.Errorf("FilterUidsStillMatching reply not array: %T", reply)
	}
	out := make([]uint32, 0, len(arr))
	for _, el := range arr {
		s, _ := el.(string)
		if s == "" {
			continue
		}
		n, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			continue
		}
		out = append(out, uint32(n))
	}
	return out, nil
}

// luaFilterStillMatching: 對 KEYS[i]（uid/status）檢查 HGET state == "matching"，若成立則該 uid 在 out 中。
// 回傳仍為 matching 的 uid 列表。KEYS[i] = "{uid}/status"，需從 key 反推 uid（key 格式為 "123/status"）。
const luaFilterStillMatching = `
local out = {}
for i = 1, #KEYS do
  local state = redis.call("HGET", KEYS[i], "state")
  if state == "matching" then
    local uid = string.match(KEYS[i], "^(%d+)/")
    if uid then
      table.insert(out, uid)
    end
  end
end
return out
`
