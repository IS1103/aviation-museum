using Cysharp.Threading.Tasks;

namespace GameLink.Libs.StateMachine
{
    /// <summary>State machine facade (aligned with TS StateMachine interface).</summary>
    public interface IStateMachine<TState, TContext>
        where TState : struct
    {
        TState GetCurrentState();
        TState? GetPreviousState();
        TContext GetContext();
        void UpdateContext(System.Func<TContext, TContext> updater);
        bool IsTransitioning();

        UniTask<TState> DoNext(TransitionOptions<TState>? options = null);
        UniTask<TState> ForceNext(TState target, TransitionOptions<TState>? options = null);

        /// <summary>Sets state without calling onExit/onEnter. Resets initial-enter flag like TS reset.</summary>
        void Reset(TState? targetState = null);
    }
}
