package ws

import (
	stdcontext "context"
	"fmt"

	"internal/logger"

	"go.uber.org/zap"
)

// SendMessageOrLog 发送 WebSocket 消息，失败时记录日志
// 返回 error 表示发送失败
func SendMessageOrLog(ctx WSContext, data []byte, logPrefix string) error {
	wsConn := GetWSConn(ctx)
	if wsConn == nil {
		logger.Error("Failed to get websocket connection",
			zap.String("prefix", logPrefix),
		)
		return fmt.Errorf("no websocket connection")
	}

	if wsCtx, ok := ctx.(*WsContext); ok {
		if err := wsCtx.SendMessage(stdcontext.Background(), data); err != nil {
			logger.Error("Failed to send message",
				zap.String("prefix", logPrefix),
				zap.Error(err),
			)
			return err
		}
	}

	return nil
}
