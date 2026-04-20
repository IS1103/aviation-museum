package handler

import (
	"context"
	"fmt"
	"strings"

	"air-museum/internal/db"
	pb "air-museum/proto/pb"
	forward "internal/grpc/forward"
	"internal/logger"
)

func init() {
	forward.Request("register", registerPlayer)
}

// registerPlayer 玩家註冊 request API：request/air_museum/register。
// 輸入：name、age、sex（PlayerInput 之外的身分建檔欄位）。
// 行為：寫入 player 表（其餘欄位吃 DB 預設值），回傳新建的 player.uid。
// 注意：ctx 帶的 connUID 是 WS 連線時 token 解析出的暫時 uid，不等於 player.uid；
// 新 player.uid 僅隨 RegisterResp 回傳，客戶端自行保存與後續流程使用。
func registerPlayer(ctx context.Context, connUID uint32, req *pb.RegisterReq) (*pb.RegisterResp, error) {
	if req == nil {
		return nil, fmt.Errorf("req is required")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	age := int(req.GetAge())
	sex := int(req.GetSex())

	newUID, err := db.CreatePlayer(ctx, name, age, sex)
	if err != nil {
		logger.GateWarnf("[air_museum] register player failed (connUID=%d): %v", connUID, err)
		return nil, err
	}

	logger.GateInfof("[air_museum] register player (connUID=%d) -> uid=%d name=%s age=%d sex=%d",
		connUID, newUID, name, age, sex)
	return &pb.RegisterResp{Uid: newUID}, nil
}
