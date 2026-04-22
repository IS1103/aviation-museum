package handler

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"air-museum/internal/db"
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

// parsedToken 為 parseToken 的結構化結果。
// 續玩：IsRegister=false，uid 來自 key=<uid>（預設 1）。
// 首登：IsRegister=true，伺服器忽略 uid，呼叫 db.CreatePlayer(Name, Age, Sex) 建檔取得新 uid。
type parsedToken struct {
	Uid         uint32
	IsProjector bool
	IsRegister  bool
	Name        string
	Age         int32
	Sex         int32
}

// parseToken 解析 token：
//   - 續玩格式：key=<uid>&device=player
//   - 投影端：   device=projector
//   - 首登格式：register&name=<url-encoded>&age=<n>&sex=<n>&device=player
//
// 回傳的 Uid 於續玩/投影端情境下有效；首登時 Uid 由呼叫端另行建檔後覆寫。
func parseToken(token string) parsedToken {
	res := parsedToken{Uid: 1}
	for _, part := range strings.Split(token, "&") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "register" {
			res.IsRegister = true
			continue
		}
		eq := strings.IndexByte(part, '=')
		if eq <= 0 {
			continue
		}
		key := part[:eq]
		val := part[eq+1:]
		switch key {
		case "key":
			if n, err := strconv.ParseUint(val, 10, 32); err == nil && n > 0 {
				res.Uid = uint32(n)
			}
		case "device":
			res.IsProjector = strings.TrimSpace(val) == "projector"
		case "name":
			if decoded, err := url.QueryUnescape(val); err == nil {
				res.Name = strings.TrimSpace(decoded)
			} else {
				res.Name = strings.TrimSpace(val)
			}
		case "age":
			if n, err := strconv.ParseInt(val, 10, 32); err == nil {
				res.Age = int32(n)
			}
		case "sex":
			if n, err := strconv.ParseInt(val, 10, 32); err == nil {
				res.Sex = int32(n)
			}
		}
	}
	return res
}

// HandleAuthValidate 本機驗證（整合註冊）：token 支援三種格式
//   - 投影端：        device=projector
//   - 玩家續玩：      key=<uid>&device=player
//   - 玩家首登註冊：  register&name=<url-encoded>&age=<n>&sex=<n>&device=player
//
// 首登時於此 handler 內呼叫 db.CreatePlayer 取得新 uid，再同步完成 SetUID / Register，避免多次 round-trip。
// 投影端通過 auth 後 SetHost、Add(uid)。
func HandleAuthValidate(ctx ws.WSContext, req *gatepb.ValidateReq) (*gatepb.ValidateResp, error) {
	if req == nil {
		return nil, fmt.Errorf("req is required")
	}
	if req.GetToken() == "" {
		return nil, fmt.Errorf("token is required")
	}

	parsed := parseToken(req.GetToken())
	uid := parsed.Uid
	isProjector := parsed.IsProjector
	// Token 內若無 device=，改以 payload 的 Device 欄位判斷（與客戶端傳入一致）
	if d := strings.TrimSpace(strings.ToLower(req.GetDevice())); d == "projector" {
		isProjector = true
	} else if d == "player" {
		isProjector = false
	}

	// 首登註冊：僅玩家端允許；投影端忽略 register 欄位，照舊固定 uid=1
	if parsed.IsRegister && !isProjector {
		if parsed.Name == "" {
			return nil, fmt.Errorf("register requires name")
		}
		newUid, err := db.CreatePlayer(context.Background(), parsed.Name, int(parsed.Age), int(parsed.Sex))
		if err != nil {
			return nil, fmt.Errorf("create player: %w", err)
		}
		uid = newUid
		logger.GateInfo(fmt.Sprintf("[air_museum] auth.register created uid=%d name=%s age=%d sex=%d",
			uid, parsed.Name, parsed.Age, parsed.Sex))
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
