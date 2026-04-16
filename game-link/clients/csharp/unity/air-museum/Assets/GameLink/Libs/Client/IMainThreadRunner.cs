// IMainThreadRunner.cs - 主線程派發介面，供 Client 將封包/回調派到主線程執行
using System;

namespace GameLink.Libs.Client
{
    /// <summary>將委派排入佇列，由實作方（如 MonoBehaviour.Update）在主線程依序執行。</summary>
    public interface IMainThreadRunner
    {
        void Enqueue(Action action);
    }
}
