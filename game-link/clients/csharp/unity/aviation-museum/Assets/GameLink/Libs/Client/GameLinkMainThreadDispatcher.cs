// GameLinkMainThreadDispatcher.cs - 主線程佇列派發，供 WebSocket 等回調安全回到 Unity 主線程
using System;
using System.Collections.Concurrent;
using UnityEngine;

namespace GameLink.Libs.Client
{
    /// <summary>
    /// 全域主線程派發單例（備用）。建議改為使用 GameLinkClientRunner 掛在 GameObject 上並注入 Client。
    /// 將委派排入佇列，並在 Unity 主線程的 Update 中依序執行。
    /// </summary>
    public class GameLinkMainThreadDispatcher : MonoBehaviour, IMainThreadRunner
    {
        private static GameLinkMainThreadDispatcher _instance;

        /// <summary>若已建立則回傳單例，否則為 null。</summary>
        public static GameLinkMainThreadDispatcher Instance => _instance;

        /// <summary>靜態入口（相容舊用法）。若尚未建立 Dispatcher 會忽略。</summary>
        public static void Enqueue(Action action) => _instance?.EnqueueInstance(action);

        /// <summary>實作 IMainThreadRunner：將 action 排入佇列。</summary>
        void IMainThreadRunner.Enqueue(Action action) => EnqueueInstance(action);

        private void EnqueueInstance(Action action)
        {
            if (action == null) return;
            _queue.Enqueue(action);
        }

        /// <summary>目前佇列中待執行的數量（僅供除錯）。</summary>
        public static int QueuedCount => _instance?._queue.Count ?? 0;

        private readonly ConcurrentQueue<Action> _queue = new ConcurrentQueue<Action>();
        private const int MaxExecutionsPerFrame = 100;

        private void Awake()
        {
            if (_instance != null && _instance != this)
            {
                Destroy(gameObject);
                return;
            }
            _instance = this;
            DontDestroyOnLoad(gameObject);
        }

        private void OnDestroy()
        {
            if (_instance == this)
                _instance = null;
        }

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

        [RuntimeInitializeOnLoadMethod(RuntimeInitializeLoadType.BeforeSceneLoad)]
        private static void CreateIfNeeded()
        {
            if (_instance != null) return;
            var go = new GameObject("GameLinkMainThreadDispatcher");
            go.AddComponent<GameLinkMainThreadDispatcher>();
        }
    }
}
