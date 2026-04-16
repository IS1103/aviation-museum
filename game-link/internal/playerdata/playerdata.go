package playerdata

import (
	"context"
	"fmt"
	"strconv"

	"internal/gateforward"

	pbgame "internal.proto/pb/game"
	gatepb "internal.proto/pb/gate"
	profilepb "internal.proto/pb/profile"

	"google.golang.org/protobuf/types/known/anypb"
)

// Fetch 將 profile 與 wallet 的資料整合為一份 pbgame.Player。
func Fetch(ctx context.Context, uid string, coinType uint32) (*pbgame.Player, error) {
	if uid == "" {
		return nil, fmt.Errorf("uid 不能為空")
	}

	// 將 string 轉換為 uint32
	uid32, err := strconv.ParseUint(uid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("無效的 uid: %w", err)
	}
	uidUint32 := uint32(uid32)

	// 先取 profile
	profile, err := fetchProfile(ctx, uidUint32)
	if err != nil {
		return nil, fmt.Errorf("取得 profile 失敗: %w", err)
	}

	// 再取 wallet
	wallet, err := fetchWallet(ctx, uidUint32, coinType)
	if err != nil {
		return nil, fmt.Errorf("取得 wallet 失敗: %w", err)
	}

	return &pbgame.Player{
		Uid:      uidUint32,
		Nickname: profile.Nickname,
		Avatar:   profile.Avatar,
		CoinType: wallet.CoinType,
		Coin:     wallet.Coin,
		Svt:      profile.Svt,
	}, nil
}

func fetchProfile(ctx context.Context, uid uint32) (*profilepb.GetProfile_Resp, error) {
	info, err := anypb.New(&gatepb.UidReq{Uid: uid})
	if err != nil {
		return nil, fmt.Errorf("anypb.New UidReq: %w", err)
	}
	respAny, err := gateforward.CallRequestWithUID(ctx, uid, "profile", "getProfile", info)
	if err != nil {
		return nil, err
	}
	if respAny == nil {
		return nil, fmt.Errorf("profile 回應為 nil")
	}
	var resp profilepb.GetProfile_Resp
	if err := respAny.UnmarshalTo(&resp); err != nil {
		return nil, fmt.Errorf("Unmarshal GetProfile_Resp: %w", err)
	}
	// 若 profile 服務未返回 uid，補上請求的 uid
	if resp.Uid == 0 {
		resp.Uid = uid
	}
	return &resp, nil
}

func fetchWallet(ctx context.Context, uid uint32, coinType uint32) (*profilepb.GetWallet_Resp, error) {
	info, err := anypb.New(&profilepb.GetWallet_Req{Uid: uid, CoinType: coinType})
	if err != nil {
		return nil, fmt.Errorf("anypb.New GetWallet_Req: %w", err)
	}
	respAny, err := gateforward.CallRequestWithUID(ctx, uid, "profile", "getWallet", info)
	if err != nil {
		return nil, err
	}
	if respAny == nil {
		return nil, fmt.Errorf("wallet 回應為 nil")
	}
	var resp profilepb.GetWallet_Resp
	if err := respAny.UnmarshalTo(&resp); err != nil {
		return nil, fmt.Errorf("Unmarshal GetWallet_Resp: %w", err)
	}
	// 若 wallet 服務未回傳 uid/coinType，補上請求的值
	if resp.Uid == 0 {
		resp.Uid = uid
	}
	if resp.CoinType == 0 {
		resp.CoinType = coinType
	}
	return &resp, nil
}
