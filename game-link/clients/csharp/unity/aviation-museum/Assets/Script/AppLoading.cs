using AirMuseum;
using Cysharp.Threading.Tasks;
using UnityEngine;
using UnityEngine.SceneManagement;
using UnityEngine.UI;

public class AppLoading : MonoBehaviour
{
    [Header("場景")]
    [SerializeField] private Slider progressSlider;
    [SerializeField] private string nextSceneName = "App";

    [Header("AirMuseum 連線")]
    [Tooltip("WebSocket 網址，需指向 air-museum 服務，例如 ws://192.168.1.100:8770/ws")]
    [SerializeField] private string wsUrl = "ws://localhost:8770/ws";

    private bool _destroyed;
    private bool _subscribed;

    private void Awake()
    {
        var svc = AirMuseumService.Instance;
        svc.OnError += OnAirMuseumError;
        _subscribed = true;
    }

    private void OnDestroy()
    {
        _destroyed = true;
        if (_subscribed)
        {
            var svc = AirMuseumService.Instance;
            svc.OnError -= OnAirMuseumError;
            _subscribed = false;
        }
    }

    private void Start()
    {
        if (progressSlider != null)
        {
            progressSlider.minValue = 0f;
            progressSlider.maxValue = 1f;
            progressSlider.value = 0f;
        }

        BootFlowAsync().Forget();
    }

    private async UniTaskVoid BootFlowAsync()
    {
        await ConnectAirMuseumAsync();
        if (_destroyed) return;

        if (!AirMuseumService.Instance.IsConnected)
        {
            Debug.LogError($"[AppLoading] 連線失敗，停止載入流程 (wsUrl={wsUrl})");
            return;
        }

        Debug.Log($"[AppLoading] 連線成功，開始載入場景 (wsUrl={wsUrl}, next={nextSceneName})");
        await LoadNextSceneAsync();
    }

    private async UniTask ConnectAirMuseumAsync()
    {
        var svc = AirMuseumService.Instance;
        if (svc.IsConnected) return;

        // 不傳 runner：AppLoading 會在切場景時被銷毀，若把 GameLinkClientRunner 掛在自己身上，
        // 切到下一個場景後回應派發會卡死。交給全域 GameLinkMainThreadDispatcher（DontDestroyOnLoad）處理。
        await svc.ConnectAsync(wsUrl);
    }

    private async UniTask LoadNextSceneAsync()
    {
        AsyncOperation op = SceneManager.LoadSceneAsync(nextSceneName);
        op.allowSceneActivation = false;

        while (!op.isDone)
        {
            if (_destroyed) return;

            // Unity 的 progress 只會跑到 0.9，之後要等 allowSceneActivation = true 才會變成 1
            float progress = Mathf.Clamp01(op.progress / 0.9f);

            if (progressSlider != null)
            {
                progressSlider.value = progress;
            }

            if (op.progress >= 0.9f)
            {
                if (progressSlider != null)
                {
                    progressSlider.value = 1f;
                }
                op.allowSceneActivation = true;
            }

            await UniTask.Yield();
        }
    }

    private void OnAirMuseumError(string msg)
    {
        if (_destroyed) return;
        Debug.LogError("[AppLoading] AirMuseum 錯誤: " + msg);
    }
}
