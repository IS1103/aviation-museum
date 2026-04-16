package game

import pbgame "internal.proto/pb/game"

// Game 遊戲房間介面（包含房間資訊和事件驅動）
// 這是最小化的介面，定義了所有遊戲必須實現的基本功能
type Game interface {
	// Entry 玩家進入房間
	Entry(player *pbgame.Player) error

	// Leave 玩家離開房間
	Leave(uid uint32) error

	// StartGame 啟動遊戲
	StartGame() error

	// Reset 重置遊戲狀態並放回池中（用於 pool 重用）
	Reset()

	// IsInGame 檢查玩家是否在遊戲中
	IsInGame(uid uint32) bool
}
