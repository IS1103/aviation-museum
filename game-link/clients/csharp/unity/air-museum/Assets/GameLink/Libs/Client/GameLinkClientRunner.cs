// GameLinkClientRunner.cs - 用 MonoBehaviour 在主線程接收並處理封包／回調
using System;
using System.Collections.Concurrent;
using UnityEngine;

namespace GameLink.Libs.Client
{
    /// <summary>
    /// 將封包與回調排入佇列，在 Unity 主線程 Update 中依序執行。
    /// 掛在 GameObject 上，並將此 runner 傳給 WebsocketClient，可避免全域靜態、生命週期與 GameObject 一致。
    /// </summary>
    public class GameLinkClientRunner : MonoBehaviour, IMainThreadRunner
    {
        private readonly ConcurrentQueue<Action> _queue = new ConcurrentQueue<Action>();
        private const int MaxExecutionsPerFrame = 100;

        /// <summary>將 action 排入佇列，會在下一幀（或之後）的 Update 中於主線程執行。</summary>
        public void Enqueue(Action action)
        {
            if (action == null) return;
            _queue.Enqueue(action);
        }

        /// <summary>目前佇列中待執行的數量（僅供除錯）。</summary>
        public int QueuedCount => _queue.Count;

        private void Update()
        {
            for (int i = 0; i < MaxExecutionsPerFrame && _queue.TryDequeue(out var action); i++)
            {
                try
                {
                    action();
                }
                catch (Exception ex)
                {
                    Debug.LogException(ex);
                }
            }
        }
    }
}
