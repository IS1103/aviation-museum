// Package routebind: {uid}/status (Hash) 與 Entry 搶佔 Lua（重構方案 10.1）
package routebind

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// keyStatus 回傳 Redis key: {uid}/status (Hash)
func keyStatus(uid uint32) string {
	return fmt.Sprintf("%d/status", uid)
}

// Entry 搶佔 Lua（10.1）：若 key 不存在則寫入 state=matching, svt, sid="", match_id=""
// KEYS[1] = "{uid}/status", ARGV[1] = svt
// 回傳：若已存在則回傳 HGETALL 的 flat array；若新建立則執行 HSET 並回傳 "OK"
const luaEntryClaim = `
local exists = redis.call("EXISTS", KEYS[1])
if exists == 1 then
  return redis.call("HGETALL", KEYS[1])
end
redis.call("HSET", KEYS[1], "state", "matching", "svt", ARGV[1], "sid", "", "match_id", "")
return {"OK"}
`

// EntryClaimResult 表示 TryClaimEntryStatus 的結果
type EntryClaimResult struct {
	Claimed  bool              // true 表示本次搶佔成功（剛寫入 matching）
	Existing map[string]string // 若 Claimed==false，為既有 status 的 Hash；若 Claimed==true 為 nil
}

// TryClaimEntryStatus 執行 Entry 搶佔 Lua：若 {uid}/status 不存在則寫入 state=matching, svt, sid="", match_id=""；若存在則不回寫，回傳既有內容。
// 呼叫方依 Existing 判斷 CASE 2/3（同遊戲 / 不同遊戲）。
func TryClaimEntryStatus(uid uint32, svt string) (*EntryClaimResult, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	svt = strings.TrimSpace(svt)
	if svt == "" {
		return nil, fmt.Errorf("svt is empty")
	}
	key := keyStatus(uid)
	reply, err := redisEval(luaEntryClaim, []string{key}, []string{svt})
	if err != nil {
		return nil, err
	}
	// reply: 若新建立為 ["OK"]；若已存在為 HGETALL 的 flat array ["state","matching","svt","holdem",...]
	arr, ok := reply.([]interface{})
	if !ok || len(arr) == 0 {
		return nil, fmt.Errorf("unexpected redis reply type")
	}
	if len(arr) == 1 {
		if s, _ := arr[0].(string); s == "OK" {
			return &EntryClaimResult{Claimed: true}, nil
		}
	}
	// flat array of key, value, key, value...
	existing := make(map[string]string)
	for i := 0; i+1 < len(arr); i += 2 {
		k, _ := arr[i].(string)
		v, _ := arr[i+1].(string)
		existing[k] = v
	}
	return &EntryClaimResult{Claimed: false, Existing: existing}, nil
}

// GetStatus 讀取 {uid}/status 整顆 Hash；若 key 不存在回傳 nil, nil。
func GetStatus(uid uint32) (map[string]string, error) {
	if uid == 0 {
		return nil, fmt.Errorf("uid is empty")
	}
	key := keyStatus(uid)
	conn, err := redisConn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	cmd := respArray("HGETALL", key)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return nil, err
	}
	br := bufio.NewReader(conn)
	reply, err := readReply(br)
	if err != nil {
		return nil, err
	}
	arr, ok := reply.([]interface{})
	if !ok || len(arr) == 0 {
		return nil, nil // key 不存在時 HGETALL 回傳空 array
	}
	out := make(map[string]string)
	for i := 0; i+1 < len(arr); i += 2 {
		k, _ := arr[i].(string)
		v, _ := arr[i+1].(string)
		out[k] = v
	}
	return out, nil
}

// DelStatus 刪除 {uid}/status（用於 Leave / Rollback）。
func DelStatus(uid uint32) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	return deleteRedisValue(keyStatus(uid))
}

// 10.2 CAS 轉 gaming：條件 state==matching && match_id==ARGV[1] && svt==ARGV[2] 則 HSET state=gaming, sid=ARGV[3]
const luaCasToGaming = `
local state = redis.call("HGET", KEYS[1], "state")
local mid   = redis.call("HGET", KEYS[1], "match_id")
local svt   = redis.call("HGET", KEYS[1], "svt")
if state ~= "matching" or mid ~= ARGV[1] or svt ~= ARGV[2] then
  return 0
end
redis.call("HSET", KEYS[1], "state", "gaming", "sid", ARGV[3])
return 1
`

// CasToGaming 執行 10.2 Lua：僅當 state==matching 且 match_id、svt 一致時寫入 state=gaming, sid=targetSid；回傳是否寫入成功。
func CasToGaming(uid uint32, matchID, svt, targetSid string) (bool, error) {
	if uid == 0 {
		return false, fmt.Errorf("uid is empty")
	}
	key := keyStatus(uid)
	reply, err := redisEval(luaCasToGaming, []string{key}, []string{matchID, svt, targetSid})
	if err != nil {
		return false, err
	}
	n, _ := reply.(int64)
	return n == 1, nil
}

// SetStatusMatchID 寫入 {uid}/status 的 match_id 欄位（Match 在 Notify 後補寫，11.7）。
func SetStatusMatchID(uid uint32, matchID string) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	key := keyStatus(uid)
	_, err := redisDo("HSET", key, "match_id", matchID)
	return err
}

// SetStatusGamingDirect 直接寫入 {uid}/status 為 state=gaming, svt, sid, match_id=""（用於 CASE 1 直接加入未滿桌，不經 Match）。
func SetStatusGamingDirect(uid uint32, svt, sid string) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	svt = strings.TrimSpace(svt)
	sid = strings.TrimSpace(sid)
	if svt == "" || sid == "" {
		return fmt.Errorf("svt and sid are required")
	}
	key := keyStatus(uid)
	_, err := redisDo("HSET", key, "state", "gaming", "svt", svt, "sid", sid, "match_id", "")
	return err
}

// 10.3 鎖過期清理：依 pending_room 取得 uids，若 {uid}/status.match_id == match_id 則 DEL status，最後 DEL pending_room
const luaRollback = `
local uids = redis.call("LRANGE", "match:pending_room:" .. ARGV[1], 0, -1)
for _, uid in ipairs(uids) do
  local cur = redis.call("HGET", uid .. "/status", "match_id")
  if cur == ARGV[1] then
    redis.call("DEL", uid .. "/status")
  end
end
redis.call("DEL", "match:pending_room:" .. ARGV[1])
return #uids
`

// RunRollbackByMatchID 執行 10.3 Lua（僅由搶到 cleanup_lock 的 Match 呼叫）；回傳被清理的 uid 數量。
func RunRollbackByMatchID(matchID string) (int, error) {
	matchID = strings.TrimSpace(matchID)
	if matchID == "" {
		return 0, fmt.Errorf("matchID is empty")
	}
	reply, err := redisEval(luaRollback, []string{}, []string{matchID})
	if err != nil {
		return 0, err
	}
	n, _ := reply.(int64)
	return int(n), nil
}

func redisConn() (net.Conn, error) {
	addr := redisAddr()
	conn, err := net.DialTimeout("tcp", addr, redisDialTimeout())
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(redisIOTimeout()))
	return conn, nil
}

// redisEval 執行 Lua 腳本；keys 與 args 對應 EVAL 的 KEYS 與 ARGV。
func redisEval(script string, keys, args []string) (interface{}, error) {
	conn, err := redisConn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// EVAL script numkeys key [key ...] arg [arg ...]
	evalArgs := make([]string, 0, 4+len(keys)+len(args))
	evalArgs = append(evalArgs, "EVAL", script, strconv.Itoa(len(keys)))
	evalArgs = append(evalArgs, keys...)
	evalArgs = append(evalArgs, args...)
	cmd := respArray(evalArgs...)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return nil, err
	}
	br := bufio.NewReader(conn)
	return readReply(br)
}

// redisDo 發送單一 Redis 指令並回傳一個 reply（供 matchkeys 等使用）。
func redisDo(args ...string) (interface{}, error) {
	conn, err := redisConn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	cmd := respArray(args...)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return nil, err
	}
	br := bufio.NewReader(conn)
	return readReply(br)
}

// readReply 從 RESP 串流讀取一個 reply，回傳 string / int64 / []interface{}。
func readReply(br *bufio.Reader) (interface{}, error) {
	line, err := br.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSuffix(line, "\r\n")
	if line == "" {
		return nil, fmt.Errorf("empty reply")
	}
	switch line[0] {
	case '+':
		return line[1:], nil
	case '-':
		return nil, fmt.Errorf("redis error: %s", line[1:])
	case ':':
		n, err := strconv.ParseInt(line[1:], 10, 64)
		if err != nil {
			return nil, err
		}
		return n, nil
	case '$':
		n, err := strconv.Atoi(line[1:])
		if err != nil {
			return nil, err
		}
		if n == -1 {
			return nil, nil // nil bulk
		}
		buf := make([]byte, n+2)
		if _, err := br.Read(buf); err != nil {
			return nil, err
		}
		return string(buf[:n]), nil
	case '*':
		n, err := strconv.Atoi(line[1:])
		if err != nil {
			return nil, err
		}
		if n == -1 {
			return nil, nil
		}
		arr := make([]interface{}, 0, n)
		for i := 0; i < n; i++ {
			el, err := readReply(br)
			if err != nil {
				return nil, err
			}
			arr = append(arr, el)
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("unexpected resp type: %q", line)
	}
}
