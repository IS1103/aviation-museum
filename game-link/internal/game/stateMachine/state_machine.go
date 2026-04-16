package stateMachine

import (
	"context"
	"sync"
	"time"
)

// StateMachine 帶 goroutine 的狀態機
type StateMachine struct {
	mu sync.RWMutex

	// 狀態機控制
	ctx        context.Context
	cancel     context.CancelFunc
	running    bool
	done       chan struct{}

	// 當前狀態
	currentState  State
	currentConfig *StateConfig

	// 狀態配置映射
	stateConfigs map[string]*StateConfig

	// 共享數據
	data interface{}

	// 轉換控制
	transitionChan chan transitionRequest
	forceNextChan  chan struct{}
}

// transitionRequest 狀態轉換請求
type transitionRequest struct {
	targetState State
	result      chan error
}

// NewStateMachine 創建新的狀態機
func NewStateMachine(ctx context.Context) *StateMachine {
	smCtx, cancel := context.WithCancel(ctx)
	return &StateMachine{
		ctx:            smCtx,
		cancel:         cancel,
		done:           make(chan struct{}),
		stateConfigs:   make(map[string]*StateConfig),
		transitionChan: make(chan transitionRequest, 1),
		forceNextChan:  make(chan struct{}, 1),
	}
}

// RegisterState 註冊狀態配置
func (sm *StateMachine) RegisterState(config *StateConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if config.State == nil {
		panic("狀態不能為 nil")
	}
	sm.stateConfigs[config.State.Name()] = config
}

// Start 啟動狀態機（從指定狀態開始）
func (sm *StateMachine) Start(initialState State, data interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.running {
		return ErrStateMachineAlreadyRunning
	}

	if initialState == nil {
		return ErrInvalidState
	}

	config, exists := sm.stateConfigs[initialState.Name()]
	if !exists {
		config = &StateConfig{State: initialState}
	}

	sm.currentState = initialState
	sm.currentConfig = config
	sm.data = data
	sm.running = true

	go sm.run()

	return nil
}

// Stop 停止狀態機（安全回收）
func (sm *StateMachine) Stop() error {
	sm.mu.Lock()
	if !sm.running {
		sm.mu.Unlock()
		return nil
	}
	sm.mu.Unlock()

	sm.cancel()

	select {
	case <-sm.done:
		return nil
	case <-time.After(5 * time.Second):
		return ErrStopTimeout
	}
}

// ForceNext 強制轉換到下一個狀態（中斷倒數，直接進入原本指定的狀態）
func (sm *StateMachine) ForceNext() error {
	sm.mu.RLock()
	if !sm.running {
		sm.mu.RUnlock()
		return ErrStateMachineNotRunning
	}
	sm.mu.RUnlock()

	select {
	case sm.forceNextChan <- struct{}{}:
		return nil
	case <-sm.ctx.Done():
		return ErrStateMachineStopped
	default:
		return ErrForceNextBusy
	}
}

// ForceNextTo 強制轉換到指定狀態（中斷倒數，直接進入指定的狀態）
func (sm *StateMachine) ForceNextTo(targetState State) error {
	sm.mu.RLock()
	if !sm.running {
		sm.mu.RUnlock()
		return ErrStateMachineNotRunning
	}
	if targetState == nil {
		sm.mu.RUnlock()
		return ErrInvalidState
	}
	sm.mu.RUnlock()

	req := transitionRequest{
		targetState: targetState,
		result:      make(chan error, 1),
	}

	select {
	case sm.transitionChan <- req:
		return <-req.result
	case <-sm.ctx.Done():
		return ErrStateMachineStopped
	default:
		return ErrTransitionBusy
	}
}

// GetCurrentState 獲取當前狀態
func (sm *StateMachine) GetCurrentState() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState
}

// IsRunning 檢查狀態機是否正在運行
func (sm *StateMachine) IsRunning() bool {
	sm.mu.RLock()
	defer sm.mu.Unlock()
	return sm.running
}

// run 狀態機主循環（在 goroutine 中運行）
func (sm *StateMachine) run() {
	defer func() {
		sm.mu.Lock()
		if sm.currentConfig != nil && sm.currentConfig.OnExit != nil {
			sm.currentConfig.OnExit(sm.ctx, sm.data)
		}
		sm.running = false
		close(sm.done)
		sm.mu.Unlock()
	}()

	if sm.currentConfig != nil && sm.currentConfig.OnEnter != nil {
		if err := sm.currentConfig.OnEnter(sm.ctx, sm.data); err != nil {
			return
		}
	}

	for {
		duration := sm.currentState.Duration()
		var timer *time.Timer
		var timerChan <-chan time.Time

		if duration > 0 {
			timer = time.NewTimer(duration)
			timerChan = timer.C
		}

		select {
		case <-sm.ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return

		case <-timerChan:
			if timer != nil {
				timer.Stop()
			}
			sm.handleTimeout()

		case <-sm.forceNextChan:
			if timer != nil {
				timer.Stop()
			}
			if sm.currentConfig != nil && sm.currentConfig.OnForceNext != nil {
				sm.currentConfig.OnForceNext(sm.ctx, sm.data)
			}
			sm.doNext()

		case req := <-sm.transitionChan:
			if timer != nil {
				timer.Stop()
			}
			if sm.currentConfig != nil && sm.currentConfig.OnForceNext != nil {
				sm.currentConfig.OnForceNext(sm.ctx, sm.data)
			}
			if err := sm.doTransition(req.targetState); err != nil {
				req.result <- err
			} else {
				req.result <- nil
			}
		}
	}
}

func (sm *StateMachine) handleTimeout() {
	sm.mu.Lock()
	config := sm.currentConfig
	sm.mu.Unlock()

	if config != nil && config.OnTimeout != nil {
		config.OnTimeout(sm.ctx, sm.data)
	}

	sm.doNext()
}

func (sm *StateMachine) doNext() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var nextState State

	config := sm.currentConfig
	if config != nil {
		if config.TransitionFunc != nil {
			nextState = config.TransitionFunc(sm.ctx, sm.currentState, sm.data)
		} else if config.NextState != nil {
			nextState = config.NextState
		}
	}

	if nextState == nil {
		sm.cancel()
		return
	}

	sm.doTransitionLocked(nextState)
}

func (sm *StateMachine) doTransition(targetState State) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.doTransitionLocked(targetState)
}

func (sm *StateMachine) doTransitionLocked(targetState State) error {
	if sm.currentConfig != nil && sm.currentConfig.OnExit != nil {
		if err := sm.currentConfig.OnExit(sm.ctx, sm.data); err != nil {
			return err
		}
	}

	sm.currentState = targetState
	if config, exists := sm.stateConfigs[targetState.Name()]; exists {
		sm.currentConfig = config
	} else {
		sm.currentConfig = &StateConfig{State: targetState}
	}

	if sm.currentConfig != nil && sm.currentConfig.OnEnter != nil {
		if err := sm.currentConfig.OnEnter(sm.ctx, sm.data); err != nil {
			return err
		}
	}

	return nil
}
