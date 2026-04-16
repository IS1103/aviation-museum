// Package push 訂閱端（§8.3）：Gate 訂閱 push:{uid}，收到後轉發至 WebSocket。

package push

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

// OnPushMessage 收到 push:{uid} 訊息時的回調，payload 為 Pack 二進制。
type OnPushMessage func(uid uint32, payload []byte)

var (
	subMu       sync.Mutex
	subClient   *redis.Client
	subPubSub   *redis.PubSub
	subCallback OnPushMessage
	subCancel   context.CancelFunc
)

// StartSubscriber 啟動訂閱端：連線 Redis 並在背景接收訊息，收到後呼叫 onMessage(uid, payload)。
// 應在 Gate 啟動時呼叫一次；ctx 用於控制 Receive 迴圈生命週期（如 context.Background()）。
func StartSubscriber(ctx context.Context, addr string, onMessage OnPushMessage) error {
	if addr == "" {
		return fmt.Errorf("push: redis addr is empty")
	}
	if onMessage == nil {
		panic("push: onMessage is nil")
	}
	subMu.Lock()
	defer subMu.Unlock()
	StopSubscriber()
	c := redis.NewClient(&redis.Options{Addr: addr})
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return err
	}
	subClient = c
	subPubSub = c.Subscribe(ctx) // 先不訂閱任何 channel，之後用 Subscribe 動態加入
	subCallback = onMessage
	ctx, subCancel = context.WithCancel(ctx)
	go receiveLoop(ctx)
	return nil
}

// StopSubscriber 停止訂閱端並關閉連線。
func StopSubscriber() {
	if subCancel != nil {
		subCancel()
		subCancel = nil
	}
	if subPubSub != nil {
		_ = subPubSub.Close()
		subPubSub = nil
	}
	if subClient != nil {
		_ = subClient.Close()
		subClient = nil
	}
	subCallback = nil
}

func receiveLoop(ctx context.Context) {
	for {
		msg, err := subPubSub.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		uid, err := parseUIDFromChannel(msg.Channel)
		if err != nil {
			continue
		}
		payload := []byte(msg.Payload)
		if subCallback != nil {
			subCallback(uid, payload)
		}
	}
}

func parseUIDFromChannel(channel string) (uint32, error) {
	if !strings.HasPrefix(channel, channelPrefix) {
		return 0, fmt.Errorf("push: not a push channel: %s", channel)
	}
	s := strings.TrimPrefix(channel, channelPrefix)
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(n), nil
}

// SubscribePush 訂閱該 uid 的 push channel，應在用戶連線（auth 成功、Register 後）呼叫。
func SubscribePush(ctx context.Context, uid uint32) error {
	subMu.Lock()
	ps := subPubSub
	subMu.Unlock()
	if ps == nil {
		return nil
	}
	ps.Subscribe(ctx, ChannelForUID(uid))
	return nil
}

// UnsubscribePush 取消訂閱該 uid 的 push channel，應在用戶斷線時呼叫。
func UnsubscribePush(ctx context.Context, uid uint32) error {
	subMu.Lock()
	ps := subPubSub
	subMu.Unlock()
	if ps == nil {
		return nil
	}
	ps.Unsubscribe(ctx, ChannelForUID(uid))
	return nil
}
