package handler

import (
	"context"
	"fmt"

	"air-museum/internal/room"
	pb "air-museum/proto/pb"
	forward "internal/grpc/forward"
	"internal/logger"
	"internal/middleware/common"
	"internal/webcore/conn"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func init() {
	forward.Notify("player", playerNotify)
	forward.Notify("state", stateNotify)
}

// playerNotify 玩家端唯一 API：notify/air_museum/player，payload 為 PlayerInput。依 action 分派：ENTRY 入桌、LEAVE 離桌、INPUT 轉發給投影端。
func playerNotify(ctx context.Context, uid uint32, req *pb.PlayerInput) error {
	r := room.Get()
	if req == nil {
		req = &pb.PlayerInput{}
	}
	switch req.GetAction() {
	case pb.Action_ACTION_ENTRY:
		if err := r.Add(uid); err != nil {
			return err
		}
		logger.GateInfo(fmt.Sprintf("[air_museum] uid=%d entry, players=%d", uid, r.PlayerCount()))
		sendPlayerToHost(ctx, r, pb.Action_ACTION_ENTRY, uid, 0, 0, 0)
	case pb.Action_ACTION_LEAVE:
		if uid == r.Host() {
			return nil
		}
		r.Remove(uid)
		logger.GateInfo(fmt.Sprintf("[air_museum] uid=%d leave, players=%d", uid, r.PlayerCount()))
		sendPlayerToHost(ctx, r, pb.Action_ACTION_LEAVE, uid, 0, 0, 0)
	case pb.Action_ACTION_INPUT:
		if !r.Contains(uid) {
			return nil
		}
		sendPlayerToHost(ctx, r, pb.Action_ACTION_INPUT, uid, req.GetAxisX(), req.GetAxisY(), req.GetSeq())
	default:
		// ACTION_UNSPECIFIED 等忽略
	}
	return nil
}

// sendPlayerToHost 組裝 PlayerInput 並以 notify/air_museum/player 送給房內主機（投影端）
func sendPlayerToHost(ctx context.Context, r *room.Room, action pb.Action, uid uint32, axisX, axisY float32, seq uint32) {
	hostUid := r.Host()
	if hostUid == 0 {
		return
	}
	out := &pb.PlayerInput{
		Action: action,
		Uid:    uid,
		AxisX:  axisX,
		AxisY:  axisY,
		Seq:    seq,
	}
	anyInfo, _ := anypb.New(out)
	pack, err := common.Builder.BuildNotifyPack("air_museum/player", anyInfo)
	if err != nil {
		return
	}
	packData, _ := proto.Marshal(pack)
	_ = conn.GetConnectionManager().SendToUser(ctx, hostUid, packData)
}

// stateNotify 投影端唯一 API：notify/air_museum/state，payload 為 GameState。僅主機可送；以 state + 房內 uids 廣播給所有玩家，非遊戲中時清掉「遊戲中斷線」名單。
func stateNotify(ctx context.Context, uid uint32, req *pb.GameState) error {
	r := room.Get()
	if uid != r.Host() {
		return nil
	}
	state := req.GetState()
	r.SetCurrentPhase(state)
	gs := &pb.GameState{
		State: state,
		Uids:  r.UIDs(),
	}
	anyInfo, _ := anypb.New(gs)
	pack, err := common.Builder.BuildNotifyPack("air_museum/state", anyInfo)
	if err != nil {
		return err
	}
	packData, _ := proto.Marshal(pack)
	r.BroadcastToRoom(ctx, packData, uid)
	if state != pb.GamePhase_GAME_PHASE_PLAYING {
		for _, disconnectedUID := range r.ClearDisconnectedInGameAndReturn() {
			r.Remove(disconnectedUID)
			logger.GateInfo(fmt.Sprintf("[air_museum] game ended, removed disconnected uid=%d", disconnectedUID))
		}
	}
	return nil
}
