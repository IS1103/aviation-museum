// Package slot 提供 slot 遊戲共用邏輯：機率（權重與加權隨機）、付線判定等。
// 僅由 slot 系列服務（如 slot-supermarket）import；internal/game 頂層不引用，避免 baccarat/holdem 將 slot 打進 binary。
package slot
