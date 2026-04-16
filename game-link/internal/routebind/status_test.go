package routebind

import (
	"testing"
)

func TestTryClaimEntryStatus_and_GetStatus_DelStatus(t *testing.T) {
	// 使用固定 uid 避免與其他測試衝突；若無 Redis 則 skip
	uid := uint32(99990001)
	svt := "holdem"

	// 先清空，確保從「不存在」開始
	_ = DelStatus(uid)

	// 1) 第一次搶佔：應成功
	r1, err := TryClaimEntryStatus(uid, svt)
	if err != nil {
		t.Skipf("Redis 可能未啟動或 REDIS_ADDR 未設，跳過: %v", err)
	}
	if !r1.Claimed {
		t.Fatalf("第一次搶佔應成功: Claimed=%v Existing=%v", r1.Claimed, r1.Existing)
	}
	if r1.Existing != nil {
		t.Fatalf("搶佔成功時 Existing 應為 nil: %v", r1.Existing)
	}

	// 2) GetStatus 應讀到剛寫入的 Hash
	hash, err := GetStatus(uid)
	if err != nil {
		t.Fatal(err)
	}
	if hash["state"] != "matching" || hash["svt"] != svt {
		t.Fatalf("GetStatus 應為 state=matching, svt=holdem: %v", hash)
	}

	// 3) 第二次搶佔（同一 uid）：應回傳已存在，不覆寫
	r2, err := TryClaimEntryStatus(uid, svt)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Claimed {
		t.Fatalf("第二次搶佔應為已存在: Claimed=%v", r2.Claimed)
	}
	if r2.Existing == nil {
		t.Fatal("已存在時 Existing 不應為 nil")
	}
	if r2.Existing["state"] != "matching" || r2.Existing["svt"] != svt {
		t.Fatalf("Existing 應為既有 status: %v", r2.Existing)
	}

	// 4) 刪除 status
	if err := DelStatus(uid); err != nil {
		t.Fatal(err)
	}
	hash2, err := GetStatus(uid)
	if err != nil {
		t.Fatal(err)
	}
	if hash2 != nil {
		t.Fatalf("DelStatus 後 GetStatus 應為 nil: %v", hash2)
	}

	// 5) 刪除後再搶佔：應再次成功
	r3, err := TryClaimEntryStatus(uid, svt)
	if err != nil {
		t.Fatal(err)
	}
	if !r3.Claimed {
		t.Fatalf("刪除後再搶佔應成功: Claimed=%v", r3.Claimed)
	}
	_ = DelStatus(uid) // 清理
}
