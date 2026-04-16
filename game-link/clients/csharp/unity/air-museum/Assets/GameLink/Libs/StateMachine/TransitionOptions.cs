namespace GameLink.Libs.StateMachine
{
    /// <summary>Optional arguments for DoNext / ForceNext (aligned with TS TransitionOptions).</summary>
    public sealed class TransitionOptions<TState>
        where TState : struct
    {
        /// <summary>When set, overrides ResolveNext for DoNext only.</summary>
        public TState? To { get; set; }

        public object? Payload { get; set; }
        public string? Reason { get; set; }
    }
}
