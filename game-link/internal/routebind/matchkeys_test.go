package routebind

import (
	"testing"
)

func TestNextMatchSeq(t *testing.T) {
	seq, err := NextMatchSeq()
	if err != nil {
		t.Skipf("Redis 可能未啟動，跳過: %v", err)
	}
	if seq < 1 {
		t.Fatalf("NextMatchSeq 應 >= 1: %d", seq)
	}
	seq2, err := NextMatchSeq()
	if err != nil {
		t.Fatal(err)
	}
	if seq2 != seq+1 {
		t.Fatalf("第二次 INCR 應為 seq+1: %d vs %d", seq2, seq+1)
	}
}

func TestPendingRoom_and_CurrentBatch(t *testing.T) {
	matchID := "m_20260215_1"
	uid := uint32(1001)

	// 清空
	_ = PendingRoomDel(matchID)

	// PendingRoomPush
	if err := PendingRoomPush(matchID, uid); err != nil {
		t.Skipf("Redis 可能未啟動，跳過: %v", err)
	}
	list, err := PendingRoomRange(matchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0] != "1001" {
		t.Fatalf("LRANGE 應為 [1001]: %v", list)
	}
	// LREM
	n, err := PendingRoomLRem(matchID, uid)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("LREM 應移除 1 個: %d", n)
	}
	list2, _ := PendingRoomRange(matchID)
	if len(list2) != 0 {
		t.Fatalf("LREM 後應為空: %v", list2)
	}
	_ = PendingRoomDel(matchID)
}

func TestCreateLock_and_CleanupLock(t *testing.T) {
	matchID := "m_20260215_99"
	ok1, err := CreateLockTryAcquire(matchID)
	if err != nil {
		t.Skipf("Redis 可能未啟動，跳過: %v", err)
	}
	if !ok1 {
		t.Fatalf("第一次 CreateLock 應成功")
	}
	ok2, err := CreateLockTryAcquire(matchID)
	if err != nil {
		t.Fatal(err)
	}
	if ok2 {
		t.Fatalf("第二次 CreateLock 應失敗（已佔用）")
	}
	// CleanupLock 同理
	c1, _ := CleanupLockTryAcquire(matchID)
	if !c1 {
		t.Fatalf("第一次 CleanupLock 應成功")
	}
	c2, _ := CleanupLockTryAcquire(matchID)
	if c2 {
		t.Fatalf("第二次 CleanupLock 應失敗")
	}
}

func TestCurrentBatchGetSet(t *testing.T) {
	svt := "holdem"
	minP, maxP := 2, 9
	matchID := "m_20260215_2"
	key := keyCurrentBatch(svt, minP, maxP)
	_ = deleteRedisValue(key)

	got, err := CurrentBatchGet(svt, minP, maxP)
	if err != nil {
		t.Skipf("Redis 可能未啟動，跳過: %v", err)
	}
	if got != "" {
		t.Fatalf("未設定前應為空: %q", got)
	}
	if err := CurrentBatchSet(svt, minP, maxP, matchID); err != nil {
		t.Fatal(err)
	}
	got, err = CurrentBatchGet(svt, minP, maxP)
	if err != nil {
		t.Fatal(err)
	}
	if got != matchID {
		t.Fatalf("CurrentBatchGet 應為 %q: %q", matchID, got)
	}
	_ = deleteRedisValue(key)
}

func TestPendingRoomRange_empty(t *testing.T) {
	// 不存在的 key LRANGE 回傳空陣列
	list, err := PendingRoomRange("nonexistent_match_id_xyz")
	if err != nil {
		t.Skipf("Redis 可能未啟動，跳過: %v", err)
	}
	if list == nil {
		t.Fatal("LRANGE 不存在的 key 回傳 [] 非 nil")
	}
	if len(list) != 0 {
		t.Fatalf("應為空: %v", list)
	}
}
