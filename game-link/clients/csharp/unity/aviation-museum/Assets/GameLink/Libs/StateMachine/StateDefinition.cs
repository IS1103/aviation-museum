using Cysharp.Threading.Tasks;

namespace GameLink.Libs.StateMachine
{
    /// <summary>Per-state hooks (aligned with TS StateDefinition).</summary>
    public sealed class StateDefinition<TState, TContext>
        where TState : struct
    {
        public System.Func<TContext, TransitionMeta<TState>, UniTask>? OnEnter { get; set; }
        public System.Func<TContext, TransitionMeta<TState>, UniTask>? OnExit { get; set; }

        /// <summary>Return null to stay on current state until an explicit transition.</summary>
        public System.Func<TContext, TState?>? ResolveNext { get; set; }
    }
}
