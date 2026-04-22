using System;
using System.Collections.Generic;
using Cysharp.Threading.Tasks;
using UnityEngine;

namespace GameLink.Libs.StateMachine
{
    /// <summary>Creates state machines (aligned with TS createStateMachine).</summary>
    public static class StateMachineFactory
    {
        public static IStateMachine<TState, TContext> Create<TState, TContext>(
            StateMachineConfig<TState, TContext> config)
            where TState : struct
        {
            return new Impl<TState, TContext>(config);
        }

        private sealed class Impl<TState, TContext> : IStateMachine<TState, TContext>
            where TState : struct
        {
            private readonly TState _initialState;
            private readonly IReadOnlyDictionary<TState, StateDefinition<TState, TContext>> _states;

            private TState _currentState;
            private TState? _previousState;
            private bool _transitioning;
            private bool _initialStateEntered;
            private TContext _context;
            private readonly object _initialLock = new object();

            public Impl(StateMachineConfig<TState, TContext> config)
            {
                _initialState = config.InitialState;
                _states = config.States;
                _context = config.Context;
                _currentState = config.InitialState;
                _previousState = null;
                _transitioning = false;
                _initialStateEntered = false;
                EnsureStateDefined(_initialState);
            }

            public TState GetCurrentState() => _currentState;

            public TState? GetPreviousState() => _previousState;

            public TContext GetContext() => _context;

            public void UpdateContext(Func<TContext, TContext> updater) =>
                _context = updater(_context);

            public bool IsTransitioning() => _transitioning;

            public void Reset(TState? targetState = null)
            {
                var next = targetState ?? _initialState;
                EnsureStateDefined(next);
                _previousState = null;
                _currentState = next;
                _transitioning = false;
                _initialStateEntered = false;
            }

            public async UniTask<TState> DoNext(TransitionOptions<TState>? options = null)
            {
                bool runInitialEnter;
                lock (_initialLock)
                {
                    runInitialEnter = !_initialStateEntered &&
                                      EqualityComparer<TState>.Default.Equals(_currentState, _initialState);
                    if (runInitialEnter)
                        _initialStateEntered = true;
                }

                if (runInitialEnter)
                {
                    var initialMeta = new TransitionMeta<TState>(null, _initialState, null, false, null);
                    if (_states.TryGetValue(_initialState, out var initDef) && initDef.OnEnter != null)
                    {
                        try
                        {
                            await initDef.OnEnter(_context, initialMeta);
                        }
                        catch (Exception ex)
                        {
                            Debug.LogError(
                                $"[StateMachine] Initial state \"{_currentState}\" OnEnter failed: {ex}");
                            throw;
                        }
                    }
                }

                while (_transitioning)
                    await UniTask.WaitUntil(() => !_transitioning);

                if (!_states.TryGetValue(_currentState, out var stateDef))
                    throw new InvalidOperationException(
                        string.Format(StateMachineMessages.CurrentStateUndefined, _currentState));

                TState? candidate = options?.To;
                if (!candidate.HasValue && stateDef.ResolveNext != null)
                    candidate = stateDef.ResolveNext(_context);

                if (!candidate.HasValue)
                    return _currentState;

                return await TransitionTo(candidate.Value, options, forced: false);
            }

            public UniTask<TState> ForceNext(TState target, TransitionOptions<TState>? options = null) =>
                TransitionTo(target, options, forced: true);

            private async UniTask<TState> TransitionTo(
                TState nextState,
                TransitionOptions<TState>? options,
                bool forced)
            {
                EnsureStateDefined(nextState);

                if (_transitioning)
                    throw new InvalidOperationException(StateMachineMessages.Busy);

                var finalMeta = new TransitionMeta<TState>(
                    _currentState,
                    nextState,
                    options?.Payload,
                    forced,
                    options?.Reason);

                _transitioning = true;
                var previous = _currentState;
                try
                {
                    if (_states.TryGetValue(previous, out var prevDef) && prevDef.OnExit != null)
                        await prevDef.OnExit(_context, finalMeta);

                    _previousState = previous;
                    _currentState = nextState;

                    if (_states.TryGetValue(nextState, out var nextDef) && nextDef.OnEnter != null)
                        await nextDef.OnEnter(_context, finalMeta);
                }
                catch (Exception)
                {
                    _currentState = previous;
                    _previousState = finalMeta.From;
                    _transitioning = false;
                    throw;
                }

                _transitioning = false;
                return _currentState;
            }

            private void EnsureStateDefined(TState state)
            {
                if (!_states.ContainsKey(state))
                    throw new InvalidOperationException(
                        string.Format(StateMachineMessages.StateNotInTable, state));
            }
        }
    }
}
