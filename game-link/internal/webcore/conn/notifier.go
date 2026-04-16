package conn

import (
	"context"

	"internal/logger"
	"internal/middleware/common"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Notifier 通知推送器（負責業務層的通知推送）
type Notifier struct {
	connManager *ConnectionManager
}

// NewNotifier 創建通知推送器
func NewNotifier(cm *ConnectionManager) *Notifier {
	return &Notifier{
		connManager: cm,
	}
}

// NotifyTo 向指定用戶推送通知
// 返回成功接收通知的用戶 ID 列表
// 如果某些玩家斷線或發送失敗，會優雅地跳過，不會移除玩家
func (n *Notifier) NotifyTo(uids []uint32, route string, data *anypb.Any) []uint32 {
	// 使用 PackBuilder 構建通知 Pack
	pack, err := common.Builder.BuildNotifyPack(route, data)
	if err != nil {
		logger.Error("Failed to build notify pack",
			zap.String("route", route),
			zap.Error(err),
		)
		return []uint32{}
	}

	// 序列化
	packData, err := proto.Marshal(pack)
	if err != nil {
		logger.Error("Failed to marshal notify pack",
			zap.String("route", route),
			zap.Error(err),
		)
		return []uint32{}
	}

	// 向所有指定用戶發送
	successUIDs := make([]uint32, 0, len(uids))
	for _, uid := range uids {
		if err := n.connManager.SendToUser(context.Background(), uid, packData); err == nil {
			successUIDs = append(successUIDs, uid)
		}
	}

	logger.Info("Notify sent to users",
		zap.String("route", route),
		zap.Int("success", len(successUIDs)),
		zap.Int("total", len(uids)),
	)
	return successUIDs
}

// NotifyServerErrorTo 向指定用戶推送服務端錯誤通知（統一使用 server/error 路由）
// 這是所有服務端錯誤的統一入口，錯誤類型通過 errMsg 前綴區分（如 DUPLICATE_LOGIN:, GAME_ERROR: 等）
// 返回成功接收錯誤通知的用戶 ID 列表
func (n *Notifier) NotifyServerErrorTo(uids []uint32, errMsg string) []uint32 {
	// 使用統一的 server/error 路由構建錯誤通知 Pack
	pack := common.Builder.BuildNotifyErrorPack("server/error", errMsg)

	// 序列化
	packData, err := proto.Marshal(pack)
	if err != nil {
		logger.Error("Failed to marshal server error pack",
			zap.String("error", errMsg),
			zap.Error(err),
		)
		return []uint32{}
	}

	// 向所有指定用戶發送
	successUIDs := make([]uint32, 0, len(uids))
	for _, uid := range uids {
		if err := n.connManager.SendToUser(context.Background(), uid, packData); err == nil {
			successUIDs = append(successUIDs, uid)
		}
	}

	logger.Warn("Server error sent to users",
		zap.String("error", errMsg),
		zap.Int("success", len(successUIDs)),
		zap.Int("total", len(uids)),
	)
	return successUIDs
}

// BroadcastNotify 向所有在線用戶廣播通知
func (n *Notifier) BroadcastNotify(route string, data *anypb.Any) int {
	// 使用 PackBuilder 構建通知 Pack
	pack, err := common.Builder.BuildNotifyPack(route, data)
	if err != nil {
		logger.Error("Failed to build notify pack for broadcast",
			zap.String("route", route),
			zap.Error(err),
		)
		return 0
	}

	// 序列化
	packData, err := proto.Marshal(pack)
	if err != nil {
		logger.Error("Failed to marshal notify pack for broadcast",
			zap.String("route", route),
			zap.Error(err),
		)
		return 0
	}

	// 廣播給所有在線用戶
	n.connManager.Broadcast(context.Background(), packData)

	onlineCount := n.connManager.GetOnlineCount()
	logger.Info("Broadcast notify sent",
		zap.String("route", route),
		zap.Int("onlineCount", onlineCount),
	)
	return onlineCount
}

// NotifyToChannel 向頻道的所有訂閱者推送通知
// 返回成功接收通知的用戶數量
// 如果某些玩家斷線或發送失敗，會記錄錯誤並優雅地跳過，不會移除玩家或取消訂閱
func (n *Notifier) NotifyToChannel(channel string, route string, data *anypb.Any) int {
	// 使用 PackBuilder 構建通知 Pack
	pack, err := common.Builder.BuildNotifyPack(route, data)
	if err != nil {
		logger.Error("Failed to build notify pack for channel",
			zap.String("channel", channel),
			zap.String("route", route),
			zap.Error(err),
		)
		return 0
	}

	logger.Info("Notify to channel",
		zap.String("channel", channel),
		zap.String("route", route),
		zap.Any("data", data),
	)

	// 序列化
	packData, err := proto.Marshal(pack)
	if err != nil {
		logger.Error("Failed to marshal notify pack for channel",
			zap.String("channel", channel),
			zap.String("route", route),
			zap.Error(err),
		)
		return 0
	}

	// 向頻道的所有訂閱者發送
	successCount := n.connManager.SendToChannel(context.Background(), channel, packData)

	return successCount
}

// NotifyDuplicateLoginTo 向指定用戶推送重複登入錯誤通知並關閉舊連接
// 這是一個專門處理重複登入的方法，會自動關閉舊連接
// 返回是否成功處理了重複登入（即是否有舊連接被關閉）
func (n *Notifier) NotifyDuplicateLoginTo(uid uint32) bool {
	// 使用統一的服務端錯誤推送方法
	errMsg := "DUPLICATE_LOGIN: 您的帳號在其他地方登入，此連線將被關閉"
	pack := common.Builder.BuildNotifyErrorPack("server/error", errMsg)

	// 序列化
	packData, err := proto.Marshal(pack)
	if err != nil {
		logger.Error("Failed to marshal duplicate login error pack",
			zap.Uint32("uid", uid),
			zap.Error(err),
		)
		// 即使序列化失敗，也嘗試關閉舊連接（不發送錯誤消息）
		return n.connManager.HandleDuplicateLogin(context.Background(), uid, nil)
	}

	// 使用 ConnectionManager 的 HandleDuplicateLogin 方法處理
	// 傳入序列化後的錯誤推送數據
	return n.connManager.HandleDuplicateLogin(context.Background(), uid, packData)
}

// 全局 Notifier 實例
var globalNotifier *Notifier

// GetNotifier 獲取全局 Notifier 實例
func GetNotifier() *Notifier {
	if globalNotifier == nil {
		globalNotifier = NewNotifier(GetConnectionManager())
	}
	return globalNotifier
}
