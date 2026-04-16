package common

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"

	gatepb "internal.proto/pb/gate"
)

// PackBuilder Pack 構建器（提供公共的 Pack 構建邏輯）
type PackBuilder struct{}

// BuildErrorPack 構建錯誤響應 Pack
func (pb *PackBuilder) BuildErrorPack(packType int32, route string, reqID int32, errMsg string) *gatepb.Pack {
	return pb.BuildErrorPackWithDetail(packType, route, reqID, errMsg, "")
}

// BuildErrorPackWithDetail 構建帶錯誤細節的錯誤響應 Pack
func (pb *PackBuilder) BuildErrorPackWithDetail(packType int32, route string, reqID int32, errMsg string, detail string) *gatepb.Pack {
	pack := &gatepb.Pack{
		PackType: uint32(packType),
		ReqId:    uint32(reqID),
		Success:  false,
		Msg:      errMsg,
		Detail:   detail,
	}
	// 從 route 解析 svt 和 method
	parts := strings.Split(route, "/")
	if len(parts) == 2 {
		pack.Svt = parts[0]
		pack.Method = parts[1]
	}
	return pack
}

// BuildSuccessPack 構建成功響應 Pack
// 統一使用 google.protobuf.Any 格式：data 必須是 *anypb.Any（可以是 nil）
func (pb *PackBuilder) BuildSuccessPack(packType int32, reqID int32, route string, data *anypb.Any) (*gatepb.Pack, error) {
	pack := &gatepb.Pack{
		PackType: uint32(packType),
		ReqId:    uint32(reqID),
		Success:  true,
		Info:     data, // 可以是 nil（可選字段）
	}
	// 從 route 解析 svt 和 method
	parts := strings.Split(route, "/")
	if len(parts) == 2 {
		pack.Svt = parts[0]
		pack.Method = parts[1]
	}
	return pack, nil
}

// BuildSuccessPackNoData 構建成功響應 Pack（沒有數據）
func (pb *PackBuilder) BuildSuccessPackNoData(packType int32, reqID int32, route string) *gatepb.Pack {
	pack := &gatepb.Pack{
		PackType: uint32(packType),
		ReqId:    uint32(reqID),
		Success:  true,
		Info:     nil, // 沒有數據
	}
	// 從 route 解析 svt 和 method
	parts := strings.Split(route, "/")
	if len(parts) == 2 {
		pack.Svt = parts[0]
		pack.Method = parts[1]
	}
	return pack
}

// BuildNotifyPack 構建通知推送 Pack
// 統一使用 google.protobuf.Any 格式：data 必須是 *anypb.Any（不能為 nil）
func (pb *PackBuilder) BuildNotifyPack(route string, data *anypb.Any) (*gatepb.Pack, error) {
	if data == nil {
		return nil, fmt.Errorf("notify data cannot be nil")
	}

	pack := &gatepb.Pack{
		PackType: 2, // 2 = notify
		ReqId:    0, // notify 不需要 reqId
		Success:  true,
		Info:     data,
	}
	// 從 route 解析 svt 和 method
	parts := strings.Split(route, "/")
	if len(parts) == 2 {
		pack.Svt = parts[0]
		pack.Method = parts[1]
	}
	return pack, nil
}

// BuildNotifyErrorPack 構建通知錯誤 Pack
func (pb *PackBuilder) BuildNotifyErrorPack(route string, errMsg string) *gatepb.Pack {
	pack := &gatepb.Pack{
		PackType: 2, // 2 = notify
		ReqId:    0, // notify 不需要 reqId
		Success:  false,
		Msg:      errMsg,
	}
	// 從 route 解析 svt 和 method
	parts := strings.Split(route, "/")
	if len(parts) == 2 {
		pack.Svt = parts[0]
		pack.Method = parts[1]
	}
	return pack
}

// 全局 PackBuilder 實例
var Builder = &PackBuilder{}
