using System.Collections.Generic;

namespace GameLink.Libs.StateMachine
{
    /// <summary>Construction config (aligned with TS StateMachineConfig).</summary>
    public sealed class StateMachineConfig<TState, TContext>
        where TState : struct
    {
        public StateMachineConfig(
            TState initialState,
            IReadOnlyDictionary<TState, StateDefinition<TState, TContext>> states,
            TContext context)
        {
            InitialState = initialState;
            States = states;
            Context = context;
        }

        public TState InitialState { get; }
        public IReadOnlyDictionary<TState, StateDefinition<TState, TContext>> States { get; }
        public TContext Context { get; }
    }
}
