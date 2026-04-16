// Package push 提供依 Redis Pub/Sub 的推播機制（§8.2）：PublishToUser 對 push:{uid} 發送 Pack 二進制。
// 呼叫方須先 Init(addr) 或 SetClient，各服務在 main 中依 config.GetRedisAddr() 注入。
package push

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	gatepb "internal.proto/pb/gate"
)

const channelPrefix = "push:"

var (
	mu     sync.RWMutex
	client *redis.Client
)

// Init 以 Redis 位址初始化共用 client，供 PublishToUser 使用。
// 若 addr 為空，則使用環境變數 REDIS_ADDR，再無則為 "localhost:6379"。
func Init(addr string) error {
	if addr = strings.TrimSpace(addr); addr == "" {
		if addr = strings.TrimSpace(os.Getenv("REDIS_ADDR")); addr == "" {
			addr = "localhost:6379"
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if client != nil {
		_ = client.Close()
	}
	client = redis.NewClient(&redis.Options{Addr: addr})
	return client.Ping(context.Background()).Err()
}

// SetClient 注入已建立好的 Redis client（測試或共用時使用）。
func SetClient(c *redis.Client) {
	mu.Lock()
	defer mu.Unlock()
	client = c
}

func getClient() *redis.Client {
	mu.RLock()
	defer mu.RUnlock()
	return client
}

// ChannelForUID 回傳該 uid 的 Redis channel 名稱。
func ChannelForUID(uid uint32) string {
	return channelPrefix + strconv.FormatUint(uint64(uid), 10)
}

// PublishToUser 對 Redis channel push:{uid} 發送 Pack 二進制；供任意 Service 推播給 Client（§8.2）。
func PublishToUser(ctx context.Context, uid uint32, pack *gatepb.Pack) error {
	if pack == nil {
		return fmt.Errorf("push: pack is nil")
	}
	data, err := proto.Marshal(pack)
	if err != nil {
		return fmt.Errorf("push: marshal pack: %w", err)
	}
	c := getClient()
	if c == nil {
		return fmt.Errorf("push: redis client not set, call Init(addr) or SetClient first")
	}
	channel := ChannelForUID(uid)
	return c.Publish(ctx, channel, data).Err()
}

// PublishToUids 對多個 uid 發送同一 Pack（迴圈呼叫 PublishToUser）。
func PublishToUids(ctx context.Context, uids []uint32, pack *gatepb.Pack) {
	for _, uid := range uids {
		_ = PublishToUser(ctx, uid, pack)
	}
}

// PublishNotifyToUids 以 routeSvt/routeMethod 與 info 組裝 Notify Pack 並推播給 uids（取代 CallGatePushToUids）。
func PublishNotifyToUids(ctx context.Context, uids []uint32, routeSvt, routeMethod string, info *anypb.Any) {
	if len(uids) == 0 || routeSvt == "" || routeMethod == "" || info == nil {
		return
	}
	pack := &gatepb.Pack{
		PackType: 2,
		ReqId:    0,
		Svt:      routeSvt,
		Method:   routeMethod,
		Success:  true,
		Info:     info,
	}
	PublishToUids(ctx, uids, pack)
}

// PublishErrorToUids 以 gate/error 與 errMsg 推播給 uids（取代 CallServicePushErrorToUids 推播給 client）。
func PublishErrorToUids(ctx context.Context, uids []uint32, errMsg string) {
	if len(uids) == 0 || errMsg == "" {
		return
	}
	errDetail := &gatepb.ErrorDetail{Msg: errMsg}
	info, err := anypb.New(errDetail)
	if err != nil {
		return
	}
	pack := &gatepb.Pack{
		PackType: 2,
		ReqId:    0,
		Svt:      "gate",
		Method:   "error",
		Success:  false,
		Msg:      errMsg,
		Info:     info,
	}
	PublishToUids(ctx, uids, pack)
}

// BuildDuplicateLoginPack 組裝 gate/duplicateLogin 踢人包（Svt="gate", Method="duplicateLogin", Msg=msg）。
// 供重複登入時 PublishToUser 推給舊連線所在 Gate，該 Gate 收包後 HandleDuplicateLogin 送包再關連線。
func BuildDuplicateLoginPack(msg string) *gatepb.Pack {
	if msg == "" {
		msg = "您的帳號已在其他裝置登入，此連線將被關閉。"
	}
	return &gatepb.Pack{
		PackType: 2,
		ReqId:    0,
		Svt:      "gate",
		Method:   "duplicateLogin",
		Success:  false,
		Msg:      msg,
	}
}

// PublishDuplicateLoginToUser 對指定 uid 發送 gate/duplicateLogin 踢人包（用於重複登入）。
func PublishDuplicateLoginToUser(ctx context.Context, uid uint32, msg string) error {
	return PublishToUser(ctx, uid, BuildDuplicateLoginPack(msg))
}

// IsDuplicateLoginPack 判斷是否為 gate/duplicateLogin 踢人包（Gate 收 push 時分支用）。
func IsDuplicateLoginPack(pack *gatepb.Pack) bool {
	return pack != nil && pack.GetSvt() == "gate" && pack.GetMethod() == "duplicateLogin"
}
