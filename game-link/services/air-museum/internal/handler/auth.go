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

func normDevice(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

// isHostDevice：遊戲主畫面（投影幕），連線為 player 種子 uid=1、入房為 host。
func isHostDevice(device string) bool {
	d := normDevice(device)
	return d == "gamescreen" || d == "projector" // projector 僅相容舊客戶端
}

func isPlayerDevice(device string) bool {
	return normDevice(device) == "player"
}

func isKnownAirMuseumDevice(device string) bool {
	return db.IsFixtureDevice(normDevice(device)) || isPlayerDevice(device)
}

// parsedToken 僅解析 token 字串（不含 ValidateReq.device；device 一律以欄位為準）。
// 選填：uid=<n> 續連；register&name=… 相容舊版首登。
type parsedToken struct {
	Uid        uint32
	IsRegister bool
	Name       string
	Age        int32
	Sex        int32
}

func parseToken(token string) parsedToken {
	res := parsedToken{}
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
		case "uid":
			if n, err := strconv.ParseUint(val, 10, 32); err == nil && n > 0 {
				res.Uid = uint32(n)
			}
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

// HandleAuthValidate：認證依 ValidateReq.device。固定終端從 DB 種子列取 uid 並綁定；device=player 走 CreatePlayer／續連。
func HandleAuthValidate(ctx ws.WSContext, req *gatepb.ValidateReq) (*gatepb.ValidateResp, error) {
	if req == nil {
		return nil, fmt.Errorf("req is required")
	}

	deviceRaw := strings.TrimSpace(req.GetDevice())
	if deviceRaw == "" {
		return nil, fmt.Errorf("device is required")
	}
	if !isKnownAirMuseumDevice(deviceRaw) {
		return nil, fmt.Errorf("unknown device %q", deviceRaw)
	}

	nd := normDevice(deviceRaw)

	var uid uint32

	if db.IsFixtureDevice(nd) {
		var err error
		uid, err = db.GetFixtureAuthUID(context.Background(), nd)
		if err != nil {
			return nil, err
		}
	} else if isPlayerDevice(deviceRaw) {
		tokenStr := strings.TrimSpace(req.GetToken())
		var parsed parsedToken
		if tokenStr != "" {
			parsed = parseToken(tokenStr)
		}

		if parsed.IsRegister {
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
		} else if parsed.Uid > 0 {
			if db.IsReservedFixtureUID(parsed.Uid) {
				return nil, fmt.Errorf("uid %d reserved for fixtures, cannot use as player token", parsed.Uid)
			}
			exists, err := db.PlayerExists(context.Background(), parsed.Uid)
			if err != nil {
				return nil, err
			}
			if exists {
				uid = parsed.Uid
			} else {
				newUid, err := db.CreatePlayer(context.Background(), "", 0, 0)
				if err != nil {
					return nil, fmt.Errorf("create player: %w", err)
				}
				uid = newUid
				logger.GateInfo(fmt.Sprintf("[air_museum] auth uid=%d not in db, issued new uid=%d", parsed.Uid, uid))
			}
		} else {
			newUid, err := db.CreatePlayer(context.Background(), "", 0, 0)
			if err != nil {
				return nil, fmt.Errorf("create player: %w", err)
			}
			uid = newUid
			logger.GateInfo(fmt.Sprintf("[air_museum] auth new session player uid=%d", uid))
		}
	} else {
		return nil, fmt.Errorf("unknown device %q", deviceRaw)
	}

	cm := conn.GetConnectionManager()
	if oldConn, hasOld := cm.GetConnection(uid); hasOld && oldConn != nil {
		_ = cm.HandleDuplicateLogin(context.Background(), uid, nil)
	}

	ctx.SetUID(uid)
	cm.Register(uid, ws.GetWSConn(ctx))
	logger.GateInfo(fmt.Sprintf("[air_museum] 連線：uid=%d（device=%s）", uid, deviceRaw))

	if isHostDevice(deviceRaw) {
		r := room.Get()
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

	return &gatepb.ValidateResp{Uid: uid}, nil
}
