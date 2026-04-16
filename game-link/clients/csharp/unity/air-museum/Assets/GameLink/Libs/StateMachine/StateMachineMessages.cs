namespace GameLink.Libs.StateMachine
{
    /// <summary>Fixed strings (TS used i18n for busy).</summary>
    public static class StateMachineMessages
    {
        public const string Busy = "State machine is busy (transition in progress).";
        public const string StateNotInTable = "State machine has no definition for state \"{0}\". Check config.States.";
        public const string CurrentStateUndefined = "Current state \"{0}\" is not defined.";
    }
}
