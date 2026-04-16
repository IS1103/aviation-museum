package stateMachine

import "errors"

var (
	// ErrStateMachineAlreadyRunning 狀態機已經在運行
	ErrStateMachineAlreadyRunning = errors.New("狀態機已經在運行")

	// ErrStateMachineNotRunning 狀態機未運行
	ErrStateMachineNotRunning = errors.New("狀態機未運行")

	// ErrStateMachineStopped 狀態機已停止
	ErrStateMachineStopped = errors.New("狀態機已停止")

	// ErrInvalidState 無效的狀態
	ErrInvalidState = errors.New("無效的狀態")

	// ErrStopTimeout 停止超時
	ErrStopTimeout = errors.New("停止狀態機超時")

	// ErrForceNextBusy 強制轉換通道忙碌
	ErrForceNextBusy = errors.New("強制轉換通道忙碌，請稍後再試")

	// ErrTransitionBusy 狀態轉換通道忙碌
	ErrTransitionBusy = errors.New("狀態轉換通道忙碌，請稍後再試")
)
