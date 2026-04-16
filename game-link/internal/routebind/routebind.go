package routebind

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Minimal Redis client for binding uid -> serviceSid.
// We avoid external deps because this repo uses local module paths that break `go get`.
//
// Redis key format:
// - uid/svt/{svt}/sid -> serviceSid

// redisAddrFunc 是獲取 Redis 地址的函數（可由各服務覆寫）
var redisAddrFunc = defaultRedisAddr

// defaultRedisAddr 從環境變數獲取 Redis 地址
func defaultRedisAddr() string {
	if v := strings.TrimSpace(os.Getenv("REDIS_ADDR")); v != "" {
		return v
	}
	return "localhost:6379"
}

// SetRedisAddrFunc 允許服務覆寫 Redis 地址獲取方式
func SetRedisAddrFunc(f func() string) {
	if f != nil {
		redisAddrFunc = f
	}
}

func redisAddr() string {
	return redisAddrFunc()
}

func redisDialTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv("REDIS_DIAL_TIMEOUT_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return 500 * time.Millisecond
}

func redisIOTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv("REDIS_IO_TIMEOUT_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return 500 * time.Millisecond
}

func keyForServiceSid(uid uint32, svt string) string {
	return fmt.Sprintf("%d/svt/%s/sid", uid, svt)
}

// getRedisValue 通用的 Redis GET 操作，返回 value 或空字串
func getRedisValue(key string) (string, error) {
	addr := redisAddr()
	conn, err := net.DialTimeout("tcp", addr, redisDialTimeout())
	if err != nil {
		return "", err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(redisIOTimeout()))

	cmd := respArray("GET", key)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return "", err
	}

	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		return "", err
	}
	// nil bulk: $-1\r\n
	if strings.HasPrefix(line, "$-1") {
		return "", nil
	}
	if strings.HasPrefix(line, "-") {
		return "", fmt.Errorf("redis error: %s", strings.TrimSpace(line))
	}
	// bulk string: $<len>\r\n<data>\r\n
	if !strings.HasPrefix(line, "$") {
		return "", fmt.Errorf("unexpected redis reply: %q", strings.TrimSpace(line))
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "$")))
	if err != nil || n < 0 {
		return "", fmt.Errorf("invalid bulk len: %q", strings.TrimSpace(line))
	}
	buf := make([]byte, n+2) // include trailing \r\n
	if _, err := br.Read(buf); err != nil {
		return "", err
	}
	val := strings.TrimSuffix(string(buf), "\r\n")
	return val, nil
}

// setRedisValue 通用的 Redis SET 操作
func setRedisValue(key, value string) error {
	addr := redisAddr()
	conn, err := net.DialTimeout("tcp", addr, redisDialTimeout())
	if err != nil {
		return fmt.Errorf("dial redis %s failed: %w", addr, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(redisIOTimeout()))

	// SET key value (no TTL, key will persist until deleted)
	cmd := respArray("SET", key, value)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return fmt.Errorf("write to redis failed: %w", err)
	}

	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read from redis failed: %w", err)
	}
	// +OK\r\n
	if strings.HasPrefix(line, "-") {
		return fmt.Errorf("redis error: %s", strings.TrimSpace(line))
	}
	if !strings.HasPrefix(line, "+OK") {
		return fmt.Errorf("unexpected redis reply: %q", strings.TrimSpace(line))
	}
	return nil
}

// deleteRedisValue 通用的 Redis DEL 操作
func deleteRedisValue(key string) error {
	addr := redisAddr()
	conn, err := net.DialTimeout("tcp", addr, redisDialTimeout())
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(redisIOTimeout()))

	cmd := respArray("DEL", key)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return err
	}

	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		return err
	}
	// :1\r\n (integer reply)
	if strings.HasPrefix(line, "-") {
		return fmt.Errorf("redis error: %s", strings.TrimSpace(line))
	}
	// DEL 返回 :0 或 :1，都視為成功
	return nil
}

// GetServiceSidByUID reads `uid/svt/{svt}/sid` from Redis.
// Returns empty string if missing/expired.
func GetServiceSidByUID(uid uint32, svt string) (string, error) {
	if uid == 0 {
		return "", fmt.Errorf("uid is empty")
	}
	svt = strings.TrimSpace(svt)
	if svt == "" {
		return "", fmt.Errorf("svt is empty")
	}
	return getRedisValue(keyForServiceSid(uid, svt))
}

// BindUIDToServiceSid writes `uid/svt/{svt}/sid -> serviceSid` (no TTL).
// Best-effort: returns error only if Redis is unreachable or replies with error.
func BindUIDToServiceSid(uid uint32, svt, serviceSid string) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	svt = strings.TrimSpace(svt)
	if svt == "" {
		return fmt.Errorf("svt is empty")
	}
	serviceSid = strings.TrimSpace(serviceSid)
	if serviceSid == "" {
		return fmt.Errorf("serviceSid is empty")
	}
	return setRedisValue(keyForServiceSid(uid, svt), serviceSid)
}

// DeleteServiceSidByUID deletes `uid/svt/{svt}/sid` from Redis.
// Best-effort: returns error only if Redis is unreachable or replies with error.
func DeleteServiceSidByUID(uid uint32, svt string) error {
	if uid == 0 {
		return fmt.Errorf("uid is empty")
	}
	svt = strings.TrimSpace(svt)
	if svt == "" {
		return fmt.Errorf("svt is empty")
	}
	return deleteRedisValue(keyForServiceSid(uid, svt))
}

// DeleteAllSvtBindingsBySid 在關閉時刪除 Redis 中屬於本實例的所有 uid→svt 綁定。
// 掃描 pattern `*svt/{svt}/sid`，僅刪除 value 等於 given sid 的 key。
// 回傳刪除的 key 數量；錯誤時 best-effort 仍會盡力刪除已掃描到的 key。
func DeleteAllSvtBindingsBySid(svt, sid string) (int, error) {
	svt = strings.TrimSpace(svt)
	sid = strings.TrimSpace(sid)
	if svt == "" || sid == "" {
		return 0, fmt.Errorf("svt and sid are required")
	}
	pattern := fmt.Sprintf("*svt/%s/sid", svt)
	return scanAndDeleteKeysByValue(pattern, sid)
}

// DeleteAllGateBindingsBySid 在關閉時刪除 Redis 中屬於本 Gate 實例的所有 uid→gate/sid 綁定。
// 掃描 pattern `*gate/sid`，僅刪除 value 等於 given sid 的 key。
func DeleteAllGateBindingsBySid(sid string) (int, error) {
	sid = strings.TrimSpace(sid)
	if sid == "" {
		return 0, fmt.Errorf("sid is required")
	}
	pattern := "*gate/sid"
	return scanAndDeleteKeysByValue(pattern, sid)
}

// scanAndDeleteKeysByValue 使用 SCAN 掃描匹配 pattern 的 key，若 value 等於 targetValue 則刪除。
func scanAndDeleteKeysByValue(pattern, targetValue string) (int, error) {
	keys, err := scanRedisKeys(pattern)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, key := range keys {
		val, err := getRedisValue(key)
		if err != nil {
			continue // best-effort：跳過無法讀取的 key
		}
		if val == targetValue {
			if err := deleteRedisValue(key); err != nil {
				continue // best-effort：記錄但不中斷
			}
			deleted++
		}
	}
	return deleted, nil
}

// ScanKeys 使用 SCAN 迭代取得所有匹配 pattern 的 key（供 Match CronJob 等使用）。
func ScanKeys(pattern string) ([]string, error) {
	return scanRedisKeys(pattern)
}

// scanRedisKeys 使用 SCAN 迭代取得所有匹配 pattern 的 key。
func scanRedisKeys(pattern string) ([]string, error) {
	addr := redisAddr()
	conn, err := net.DialTimeout("tcp", addr, redisDialTimeout())
	if err != nil {
		return nil, fmt.Errorf("dial redis %s failed: %w", addr, err)
	}
	defer conn.Close()

	var keys []string
	cursor := "0"
	for {
		_ = conn.SetDeadline(time.Now().Add(redisIOTimeout()))
		cmd := respArray("SCAN", cursor, "MATCH", pattern, "COUNT", "100")
		if _, err := conn.Write([]byte(cmd)); err != nil {
			return keys, fmt.Errorf("write redis failed: %w", err)
		}
		nextCursor, batch, err := parseScanResponse(conn)
		if err != nil {
			return keys, err
		}
		keys = append(keys, batch...)
		if nextCursor == "0" {
			break
		}
		cursor = nextCursor
	}
	return keys, nil
}

// parseScanResponse 解析 SCAN 回傳的 *2 (cursor, keys array)。
func parseScanResponse(conn net.Conn) (string, []string, error) {
	br := bufio.NewReader(conn)
	// *2\r\n
	line, err := br.ReadString('\n')
	if err != nil {
		return "", nil, err
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "*") {
		return "", nil, fmt.Errorf("unexpected scan reply: %q", line)
	}
	// $1\r\n0\r\n 或 $2\r\n12\r\n (cursor)
	cursorLine, err := br.ReadString('\n')
	if err != nil {
		return "", nil, err
	}
	cursorLine = strings.TrimSpace(cursorLine)
	if !strings.HasPrefix(cursorLine, "$") {
		return "", nil, fmt.Errorf("unexpected cursor len: %q", cursorLine)
	}
	cursorLen, _ := strconv.Atoi(strings.TrimPrefix(cursorLine, "$"))
	cursorBuf := make([]byte, cursorLen+2)
	if _, err := br.Read(cursorBuf); err != nil {
		return "", nil, err
	}
	nextCursor := strings.TrimSuffix(string(cursorBuf), "\r\n")

	// keys array: *N\r\n then N bulk strings
	arrLine, err := br.ReadString('\n')
	if err != nil {
		return nextCursor, nil, err
	}
	arrLine = strings.TrimSpace(arrLine)
	if !strings.HasPrefix(arrLine, "*") {
		return nextCursor, nil, nil
	}
	n, _ := strconv.Atoi(strings.TrimPrefix(arrLine, "*"))
	keys := make([]string, 0, n)
	for i := 0; i < n; i++ {
		lenLine, err := br.ReadString('\n')
		if err != nil {
			return nextCursor, keys, err
		}
		lenLine = strings.TrimSpace(lenLine)
		if lenLine == "$-1" {
			continue
		}
		strLen, _ := strconv.Atoi(strings.TrimPrefix(lenLine, "$"))
		buf := make([]byte, strLen+2)
		if _, err := br.Read(buf); err != nil {
			return nextCursor, keys, err
		}
		keys = append(keys, strings.TrimSuffix(string(buf), "\r\n"))
	}
	return nextCursor, keys, nil
}

func respArray(args ...string) string {
	var b strings.Builder
	b.Grow(64 + len(args)*16)
	b.WriteString("*")
	b.WriteString(strconv.Itoa(len(args)))
	b.WriteString("\r\n")
	for _, a := range args {
		b.WriteString("$")
		b.WriteString(strconv.Itoa(len(a)))
		b.WriteString("\r\n")
		b.WriteString(a)
		b.WriteString("\r\n")
	}
	return b.String()
}
