package stateMachine

import (
	"context"
	"time"
)

// State 狀態介面，每個狀態都需要實現這些方法
type State interface {
	// Name 返回狀態名稱
	Name() string

	// Duration 返回該狀態應該持續的時間，返回 0 表示不自動轉換
	Duration() time.Duration
}

// TransitionHandler 狀態轉換處理函數
// 返回下一個狀態，如果返回 nil 則停止狀態機
type TransitionHandler func(ctx context.Context, current State, data interface{}) State

// StateConfig 狀態配置
type StateConfig struct {
	State          State
	NextState      State                                           // 下一個狀態（可選，如果設置則自動轉換）
	TransitionFunc TransitionHandler                               // 自定義轉換函數（優先級高於 NextState）
	OnEnter        func(ctx context.Context, data interface{}) error // 進入狀態時調用
	OnExit         func(ctx context.Context, data interface{}) error // 離開狀態時調用
	OnTimeout      func(ctx context.Context, data interface{})      // 超時回調
	OnForceNext    func(ctx context.Context, data interface{})      // 強制轉換回調
}

// SimpleState 簡單狀態實現，用於快速創建狀態
type SimpleState struct {
	name     string
	duration time.Duration
}

// NewSimpleState 創建簡單狀態
func NewSimpleState(name string, duration time.Duration) *SimpleState {
	return &SimpleState{
		name:     name,
		duration: duration,
	}
}

func (s *SimpleState) Name() string {
	return s.name
}

func (s *SimpleState) Duration() time.Duration {
	return s.duration
}
