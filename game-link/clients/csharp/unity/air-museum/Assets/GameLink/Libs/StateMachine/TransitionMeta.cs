namespace GameLink.Libs.StateMachine
{
    /// <summary>Metadata passed to onExit / onEnter (aligned with TS TransitionMeta).</summary>
    public readonly struct TransitionMeta<TState>
        where TState : struct
    {
        public TransitionMeta(TState? from, TState to, object? payload, bool forced, string? reason)
        {
            From = from;
            To = to;
            Payload = payload;
            Forced = forced;
            Reason = reason;
        }

        /// <summary>Previous state; null on first DoNext initial enter.</summary>
        public TState? From { get; }

        public TState To { get; }
        public object? Payload { get; }
        public bool Forced { get; }
        public string? Reason { get; }
    }
}
