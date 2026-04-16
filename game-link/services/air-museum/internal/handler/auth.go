package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"air-museum/internal/room"
	"internal/gateforward"
	"internal/logger"
	"internal/middleware/common"
	"internal/webcore/conn"
	"internal/webcore/ws"

	gatepb "internal.proto/pb/gate"

	"google.golang.org/protobuf/proto"
)

func init() {
	gateforward.RegisterGateRoutes(HandleAuthValidate)
}

// parseToken 簡易解析 token：key=<uid>、device=projector|player；回傳 uid 與是否為投影端。
func parseToken(token string) (uid uint32, isProjector bool) {
	uid = 1
	isProjector = false
	for _, part := range strings.Split(token, "&") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "key=") {
			if n, err := strconv.ParseUint(strings.TrimPrefix(part, "key="), 10, 32); err == nil && n > 0 {
				uid = uint32(n)
			}
		}
		if strings.HasPrefix(part, "device=") {
			isProjector = strings.TrimSpace(strings.TrimPrefix(part, "device=")) == "projector"
		}
	}
	return uid, isProjector
}

// HandleAuthValidate 本機驗證：token 內含 key=uid、device=projector|player；投影端 auth 後 SetHost、Add(uid)。
func HandleAuthValidate(ctx ws.WSContext, req *gatepb.ValidateReq) (*gatepb.ValidateResp, error) {
	if req == nil {
		return nil, fmt.Errorf("req is required")
	}
	if req.GetToken() == "" {
		return nil, fmt.Errorf("token is required")
	}

	uid, isProjector := parseToken(req.GetToken())
	// Token 內若無 device=，改以 payload 的 Device 欄位判斷（與客戶端傳入一致）
	if d := strings.TrimSpace(strings.ToLower(req.GetDevice())); d == "projector" {
		isProjector = true
	} else if d == "player" {
		isProjector = false
	}
	// 主機端 auth 不帶 uid（投影端沒有 key=uid），由服務端固定指派；玩家端則用 token 的 key=uid，缺則預設 1
	if isProjector {
		uid = 1
	} else if uid == 0 {
		uid = 1
	}

	// 同機重複登入：本機踢舊連線（直連無 Redis push）
	cm := conn.GetConnectionManager()
	if oldConn, hasOld := cm.GetConnection(uid); hasOld && oldConn != nil {
		_ = cm.HandleDuplicateLogin(context.Background(), uid, nil)
	}

	ctx.SetUID(uid)
	cm.Register(uid, ws.GetWSConn(ctx))

	if isProjector {
		r := room.Get()
		// 主機重連：只踢房內玩家，送 error 後關閉連線，讓玩家重新 entry
		hostUid := uid
		for _, u := range r.UIDs() {
			if u == hostUid {
				continue
			}
			pack := common.Builder.BuildNotifyErrorPack("air_museum/error", "主機已重連，請點擊登入重新 entry")
			if pack != nil {
				data, _ := proto.Marshal(pack)
				cm.CloseUserWithNotify(context.Background(), u, data)
			}
			r.Remove(u)
		}
		r.SetHost(uid)
		_ = r.Add(uid)
	}

	logger.GateInfo(fmt.Sprintf("[%d] auth.validate success (local), projector=%v, online: %d", uid, isProjector, cm.GetOnlineCount()))
	return &gatepb.ValidateResp{Uid: uid}, nil
}
