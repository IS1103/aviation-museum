package gateforward

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	svcerr "internal/errors"
	grpcclient "internal/grpc/client"
	"internal/push"
	"internal/routebind"

	gatepb "internal.proto/pb/gate"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

// defaultCallerSvt 發起 gRPC 呼叫的服務名（用於日誌 grpc/caller/svc/method/route），由各 service 在 main 呼叫 SetDefaultCallerSvt(config.GetSvt()) 設定。
var defaultCallerSvt string

// SetDefaultCallerSvt 設定本進程的 caller 服務名，供 outgoing gRPC metadata 帶上 x-caller-svt。
func SetDefaultCallerSvt(svt string) {
	defaultCallerSvt = svt
}

// getOrGenTraceID 從 ctx 取 trace_id，若無則生成新 ID（svc-xxx 格式），確保每條 RPC 都有 trace_id。
func getOrGenTraceID(ctx context.Context) string {
	if tid := svcerr.GetTraceID(ctx); tid != "" {
		return tid
	}
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "svc-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return "svc-" + hex.EncodeToString(b)
}

// ForwardRequest 依 uid→sid 轉發 request 到後端服務。
func ForwardRequest(traceId string, uid uint32, pack *gatepb.Pack) (*anypb.Any, error) {
	svt := pack.GetSvt()
	method := pack.GetMethod()
	if svt == "" || method == "" {
		return nil, fmt.Errorf("svt and method are required")
	}

	conn, err := getConn(uid, svt)
	if err != nil || conn == nil {
		return nil, fmt.Errorf("no available instance for svt=%s: %w", svt, err)
	}

	grpcCtx := buildGRPCContext(traceId, uid, method)
	forwardCli := gatepb.NewGateServiceClient(conn)
	ensurePackSvtMethod(pack, svt, method)
	anyResp, err := forwardCli.Request(grpcCtx, pack)
	if err != nil {
		return nil, err
	}
	// 11.6：entry 成功回應時，以 Redis {uid}/status 填寫 svt（權威來源）
	if uid != 0 && method == "entry" && anyResp != nil {
		var info gatepb.EntryAndLeaveInfo
		if err := anyResp.UnmarshalTo(&info); err == nil {
			if st, _ := routebind.GetStatus(uid); st != nil && st["svt"] != "" {
				info.Svt = st["svt"]
				anyResp, _ = anypb.New(&info)
			}
		}
	}
	return anyResp, nil
}

// ForwardNotify 轉發 notify。
func ForwardNotify(traceId string, uid uint32, pack *gatepb.Pack) error {
	svt := pack.GetSvt()
	method := pack.GetMethod()
	if svt == "" || method == "" {
		return fmt.Errorf("svt and method are required")
	}

	conn, err := getConn(uid, svt)
	if err != nil || conn == nil {
		return fmt.Errorf("no available instance for svt=%s: %w", svt, err)
	}

	grpcCtx := buildGRPCContext(traceId, uid, method)
	forwardCli := gatepb.NewGateServiceClient(conn)
	ensurePackSvtMethod(pack, svt, method)
	_, err = forwardCli.Notify(grpcCtx, pack)
	return err
}

// ForwardTrigger 轉發 trigger。
func ForwardTrigger(traceId string, uid uint32, pack *gatepb.Pack) error {
	svt := pack.GetSvt()
	method := pack.GetMethod()
	if svt == "" || method == "" {
		return fmt.Errorf("svt and method are required")
	}

	conn, err := getConn(uid, svt)
	if err != nil || conn == nil {
		return fmt.Errorf("no available instance for svt=%s: %w", svt, err)
	}

	grpcCtx := buildGRPCContext(traceId, uid, method)
	forwardCli := gatepb.NewGateServiceClient(conn)
	_, err = forwardCli.Trigger(grpcCtx, &emptypb.Empty{})
	return err
}

// ForwardFetch 轉發 fetch。
func ForwardFetch(traceId string, uid uint32, pack *gatepb.Pack) (*anypb.Any, error) {
	svt := pack.GetSvt()
	method := pack.GetMethod()
	if svt == "" || method == "" {
		return nil, fmt.Errorf("svt and method are required")
	}

	conn, err := getConn(uid, svt)
	if err != nil || conn == nil {
		return nil, fmt.Errorf("no available instance for svt=%s: %w", svt, err)
	}

	grpcCtx := buildGRPCContext(traceId, uid, method)
	forwardCli := gatepb.NewGateServiceClient(conn)
	return forwardCli.Fetch(grpcCtx, &emptypb.Empty{})
}

// getConn 依 pack 的 svt 轉發：有 uid 綁定 sid 則轉該實例，否則依 svt 選實例。不依 state 轉 Match（業務邏輯由各服務處理）。
func getConn(uid uint32, svt string) (*grpc.ClientConn, error) {
	status, err := routebind.GetStatus(uid)
	if err == nil && status != nil {
		if sid := status["sid"]; sid != "" && status["svt"] == svt {
			if c, e := grpcclient.GetConnBySid(sid); e == nil && c != nil {
				return c, nil
			}
		}
	}
	return grpcclient.GetConn(svt)
}

// getConnBySvt 供 service 呼叫 service 使用，僅依 svt 取得連線（不查 uid 綁定）。
func getConnBySvt(svt string) (*grpc.ClientConn, error) {
	return grpcclient.GetConn(svt)
}

// GetConnAndSid 依 svt 取得一條連線與該實例的 sid（格式 svt-instanceID），供 Match 在 CAS 後對該實例呼叫 createRoom。
func GetConnAndSid(svt string) (*grpc.ClientConn, string, error) {
	conn, instanceID, err := grpcclient.PickConnectionWithInstanceID(svt)
	if err != nil || conn == nil {
		return nil, "", err
	}
	sid := svt
	if instanceID != "" {
		sid = svt + "-" + instanceID
	} else {
		sid = svt + "-1"
	}
	return conn, sid, nil
}

// CallServiceRequestWithConn 對指定 conn 發送 Request（用於 Match 對已選實例呼叫 createRoom）。
func CallServiceRequestWithConn(ctx context.Context, conn *grpc.ClientConn, svt string, method string, info *anypb.Any, timeout time.Duration) (*anypb.Any, error) {
	if conn == nil || svt == "" || method == "" {
		return nil, fmt.Errorf("conn, svt and method are required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	pack := &gatepb.Pack{Svt: svt, Method: method, Info: info}
	ensurePackSvtMethod(pack, svt, method)
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tid := getOrGenTraceID(ctx)
	md := metadata.Pairs("x-call-source", "service", "method", method, "trace_id", tid, "x-caller-svt", defaultCallerSvt)
	grpcCtx := metadata.NewOutgoingContext(callCtx, md)
	forwardCli := gatepb.NewGateServiceClient(conn)
	return forwardCli.Request(grpcCtx, pack)
}

func buildGRPCContext(traceId string, uid uint32, method string) context.Context {
	grpcCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_ = cancel
	uidStr := strconv.FormatUint(uint64(uid), 10)
	md := metadata.Pairs("trace_id", traceId, "uid", uidStr, "method", method, "x-caller-svt", defaultCallerSvt)
	return metadata.NewOutgoingContext(grpcCtx, md)
}

// CallRequestWithUID 供 A service 代「某 uid」向 B service 發 Request（例如 holdem 代玩家向 match/entry）。
// 會依 uid→sid 選實例、metadata 帶 uid，timeout 繼承自 ctx（預設 5s）。
func CallRequestWithUID(ctx context.Context, uid uint32, svt string, method string, info *anypb.Any) (*anypb.Any, error) {
	if svt == "" || method == "" {
		return nil, fmt.Errorf("svt and method are required")
	}
	conn, err := getConn(uid, svt)
	if err != nil || conn == nil {
		return nil, fmt.Errorf("no available instance for svt=%s: %w", svt, err)
	}
	pack := &gatepb.Pack{Svt: svt, Method: method, Info: info}
	ensurePackSvtMethod(pack, svt, method)
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	uidStr := strconv.FormatUint(uint64(uid), 10)
	tid := getOrGenTraceID(ctx)
	md := metadata.Pairs("uid", uidStr, "method", method, "trace_id", tid, "x-caller-svt", defaultCallerSvt)
	grpcCtx := metadata.NewOutgoingContext(callCtx, md)
	forwardCli := gatepb.NewGateServiceClient(conn)
	return forwardCli.Request(grpcCtx, pack)
}

// CallNotifyWithUID 供 A service 代「某 uid」向 B service 發 Notify（fire-and-forget，不等待回應）。
// 例如 holdem 代玩家向 match/entry 報到湊桌。
func CallNotifyWithUID(ctx context.Context, uid uint32, svt string, method string, info *anypb.Any) error {
	if svt == "" || method == "" {
		return fmt.Errorf("svt and method are required")
	}
	conn, err := getConn(uid, svt)
	if err != nil || conn == nil {
		return fmt.Errorf("no available instance for svt=%s: %w", svt, err)
	}
	pack := &gatepb.Pack{Svt: svt, Method: method, Info: info}
	ensurePackSvtMethod(pack, svt, method)
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	uidStr := strconv.FormatUint(uint64(uid), 10)
	tid := getOrGenTraceID(ctx)
	md := metadata.Pairs("uid", uidStr, "method", method, "trace_id", tid, "x-caller-svt", defaultCallerSvt)
	grpcCtx := metadata.NewOutgoingContext(callCtx, md)
	forwardCli := gatepb.NewGateServiceClient(conn)
	_, err = forwardCli.Notify(grpcCtx, pack)
	return err
}

// CallServiceRequestWithInfo 供 A service 呼叫 B service 使用（不帶 uid）：依 svt 選實例、metadata 帶 x-call-source: service。
// 例如 match→holdem CreateRoom。timeout 預設 5s。
func CallServiceRequestWithInfo(ctx context.Context, svt string, method string, info *anypb.Any) (*anypb.Any, error) {
	return CallServiceRequestWithInfoTimeout(ctx, svt, method, info, 5*time.Second)
}

// CallServiceRequestWithInfoTimeout 同上，可指定 timeout。createRoom 等較耗時操作請用較長 timeout（如 30s）。
func CallServiceRequestWithInfoTimeout(ctx context.Context, svt string, method string, info *anypb.Any, timeout time.Duration) (*anypb.Any, error) {
	if svt == "" || method == "" {
		return nil, fmt.Errorf("svt and method are required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	conn, err := getConnBySvt(svt)
	if err != nil || conn == nil {
		return nil, fmt.Errorf("no available instance for svt=%s: %w", svt, err)
	}
	pack := &gatepb.Pack{Svt: svt, Method: method, Info: info}
	ensurePackSvtMethod(pack, svt, method)
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tid := getOrGenTraceID(ctx)
	md := metadata.Pairs("x-call-source", "service", "method", method, "trace_id", tid, "x-caller-svt", defaultCallerSvt)
	grpcCtx := metadata.NewOutgoingContext(callCtx, md)
	forwardCli := gatepb.NewGateServiceClient(conn)
	return forwardCli.Request(grpcCtx, pack)
}

// CallServiceNotifyWithInfo 供 A service 呼叫 B service 使用（不帶 uid、fire-and-forget）：依 svt 選實例，呼叫 Notify，不取回傳。
func CallServiceNotifyWithInfo(ctx context.Context, svt string, method string, info *anypb.Any) error {
	if svt == "" || method == "" {
		return fmt.Errorf("svt and method are required")
	}
	conn, err := getConnBySvt(svt)
	if err != nil || conn == nil {
		return fmt.Errorf("no available instance for svt=%s: %w", svt, err)
	}
	pack := &gatepb.Pack{Svt: svt, Method: method, Info: info}
	ensurePackSvtMethod(pack, svt, method)
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tid := getOrGenTraceID(ctx)
	md := metadata.Pairs("x-call-source", "service", "method", method, "trace_id", tid, "x-caller-svt", defaultCallerSvt)
	grpcCtx := metadata.NewOutgoingContext(callCtx, md)
	forwardCli := gatepb.NewGateServiceClient(conn)
	_, err = forwardCli.Notify(grpcCtx, pack)
	return err
}

func ensurePackSvtMethod(pack *gatepb.Pack, svt, method string) {
	if pack.Svt == "" {
		pack.Svt = svt
	}
	if pack.Method == "" {
		pack.Method = method
	}
}

// CallServicePushToUids 對目標服務 svt 發出 pushToUids（Notify），fire-and-forget。
// 一般推播給客戶端請用 CallGatePushToUids，語義為「client 看到的 route」。
func CallServicePushToUids(ctx context.Context, uids []uint32, svt string, method string, info *anypb.Any) {
	if len(uids) == 0 || svt == "" || method == "" || info == nil {
		return
	}
	req := &gatepb.PushToUidsReq{Uids: uids, Svt: svt, Method: method, Data: info}
	payloadAny, err := anypb.New(req)
	if err != nil {
		return
	}
	_ = CallServiceNotifyWithInfo(ctx, svt, "pushToUids", payloadAny)
}

// CallGatePushToUids 經 Redis push:{uid} 推播給指定 uids，client 端收到的 route 為 routeSvt/routeMethod（§8.2）。
func CallGatePushToUids(ctx context.Context, uids []uint32, routeSvt, routeMethod string, info *anypb.Any) {
	push.PublishNotifyToUids(ctx, uids, routeSvt, routeMethod, info)
}

// CallServicePushErrorToUids 經 Redis push:{uid} 推播 gate/error 給指定 uids（§8.2）。
func CallServicePushErrorToUids(ctx context.Context, uids []uint32, svt string, errMsg string) {
	push.PublishErrorToUids(ctx, uids, errMsg)
}

// PushErrorToUids 推送錯誤訊息給指定玩家（gate/error）。遊戲 entry/leave 等通用錯誤推播用。
func PushErrorToUids(ctx context.Context, uids []uint32, msg string) {
	if len(uids) == 0 {
		return
	}
	CallServicePushErrorToUids(ctx, uids, "gate", msg)
}

// CallServiceKickUsers 對目標服務 svt 發出 kickUsers（Request），回傳 KickUserResp（含未踢成功的 uids）。
func CallServiceKickUsers(ctx context.Context, uids []uint32, svt string) (*gatepb.KickUserResp, error) {
	if len(uids) == 0 || svt == "" {
		return &gatepb.KickUserResp{Ok: true, Uids: nil}, nil
	}
	req := &gatepb.KickUserReq{Uids: uids}
	payloadAny, err := anypb.New(req)
	if err != nil {
		return nil, err
	}
	respAny, err := CallServiceRequestWithInfo(ctx, svt, "kickUsers", payloadAny)
	if err != nil {
		return nil, err
	}
	if respAny == nil {
		return &gatepb.KickUserResp{}, nil
	}
	resp := &gatepb.KickUserResp{}
	if err := respAny.UnmarshalTo(resp); err != nil {
		return nil, fmt.Errorf("unmarshal KickUserResp: %w", err)
	}
	return resp, nil
}

// CallServiceKickUsersBySid 對指定 gateSid 的 Gate 發出 kickUsers（Request），用於跨 Gate 踢人（如重複登入）。
func CallServiceKickUsersBySid(ctx context.Context, uids []uint32, gateSid string) (*gatepb.KickUserResp, error) {
	if len(uids) == 0 || gateSid == "" {
		return &gatepb.KickUserResp{Ok: true, Uids: nil}, nil
	}
	conn, err := grpcclient.GetConnBySid(gateSid)
	if err != nil || conn == nil {
		return nil, fmt.Errorf("no connection to gate sid=%s: %w", gateSid, err)
	}
	req := &gatepb.KickUserReq{Uids: uids}
	payloadAny, err := anypb.New(req)
	if err != nil {
		return nil, err
	}
	pack := &gatepb.Pack{Svt: "gate", Method: "kickUsers", Info: payloadAny}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tid := getOrGenTraceID(ctx)
	md := metadata.Pairs("x-call-source", "service", "method", "kickUsers", "trace_id", tid, "x-caller-svt", defaultCallerSvt)
	grpcCtx := metadata.NewOutgoingContext(callCtx, md)
	cli := gatepb.NewGateServiceClient(conn)
	respAny, err := cli.Request(grpcCtx, pack)
	if err != nil {
		return nil, err
	}
	if respAny == nil {
		return &gatepb.KickUserResp{}, nil
	}
	resp := &gatepb.KickUserResp{}
	if err := respAny.UnmarshalTo(resp); err != nil {
		return nil, fmt.Errorf("unmarshal KickUserResp: %w", err)
	}
	return resp, nil
}
