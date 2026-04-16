package grpcclient

import (
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	igrpc "internal/grpc"
)

// GetConn 依 service name (svt) 取得一條連線，使用內建的連線池與服務發現。
func GetConn(svt string) (*grpc.ClientConn, error) {
	return igrpc.PickConnection(svt)
}

// PickConnectionWithInstanceID 選擇一條連線並回傳實例 ID（供 Match CAS 後對該實例 createRoom）。
func PickConnectionWithInstanceID(svt string) (*grpc.ClientConn, string, error) {
	return igrpc.PickConnectionWithInstanceID(svt)
}

// GetConnBySid 依 sid（約定格式: {svt}-{instanceId}）優先取對應實例的連線。
// 若找不到或連線非 Ready，會退回使用 GetConn(svt) 走服務發現。
func GetConnBySid(sid string) (*grpc.ClientConn, error) {
	svt := parseSvtFromSid(sid)
	if svt == "" {
		return nil, fmt.Errorf("invalid sid: %s", sid)
	}

	// 使用 PickConnectionByInstance，它會自動處理動態連接
	conn, err := igrpc.PickConnectionByInstance(svt, sid)
	if err == nil && conn != nil {
		state := conn.GetState()
		if state == connectivity.Ready || state == connectivity.Idle {
			return conn, nil
		}
	}

	// fallback by svt（如果指定實例連接失敗，使用負載均衡）
	return GetConn(svt)
}

func parseSvtFromSid(sid string) string {
	if sid == "" {
		return ""
	}
	parts := strings.SplitN(sid, "-", 2)
	return parts[0]
}
