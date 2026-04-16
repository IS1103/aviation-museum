// Package routebind: Redis keyspace 訂閱 __keyevent@0__:expired（需 notify-keyspace-events Ex）
package routebind

import (
	"bufio"
	"context"
	"strings"
	"time"
)

const expiredChannel = "__keyevent@0__:expired"

// RunSubscribeExpired 訂閱 Redis 過期事件並對每個過期 key 呼叫 onKey。阻塞直到 ctx.Done。
// Redis 需設定 notify-keyspace-events Ex。
func RunSubscribeExpired(ctx context.Context, onKey func(key string)) error {
	conn, err := redisConn()
	if err != nil {
		return err
	}
	defer conn.Close()
	cmd := respArray("SUBSCRIBE", expiredChannel)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return err
	}
	br := bufio.NewReader(conn)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_ = conn.SetDeadline(time.Now().Add(redisIOTimeout()))
		reply, err := readReply(br)
		if err != nil {
			return err
		}
		arr, ok := reply.([]interface{})
		if !ok || len(arr) < 3 {
			continue
		}
		if typ, _ := arr[0].(string); typ != "message" {
			continue
		}
		if k, ok := arr[2].(string); ok && k != "" {
			onKey(k)
		}
	}
}

// ParseCreateLockExpiredKey 若 key 為 match:create_lock:{match_id} 則回傳 match_id，否則回傳空。
func ParseCreateLockExpiredKey(key string) string {
	const prefix = "match:create_lock:"
	if strings.HasPrefix(key, prefix) {
		return key[len(prefix):]
	}
	return ""
}
