using AirMuseum;
using Cysharp.Threading.Tasks;
using UnityEngine;
using UnityEngine.SceneManagement;
using UnityEngine.UI;

public class AppLoading : MonoBehaviour
{
    [Header("場景")]
    [SerializeField] private Scrollbar progressScrollbar;
    [SerializeField] private string nextSceneName = "App";

    [Tooltip("ScrollBar 追趕實際載入進度的速度（每秒 0～1）。場景載入很快時較大值可更快滿條；最後一段會自動加快避免卡死。")]
    [SerializeField] private float progressCatchUpSpeed = 2.5f;

    [Tooltip("條顯示 100% 後，再等幾幀才 allowSceneActivation，確保畫面有先畫出滿條。")]
    [SerializeField] private int framesToShowFullBarBeforeActivate = 2;

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

    // 進度條：size = 0～1（已載比例），value 固定 0 讓填色自軌道起點長出（Horizontal LTR 預設為由左填滿）。
    private void ApplyProgressScrollbarVisual(float progress01)
    {
        if (progressScrollbar == null) return;

        float p = Mathf.Clamp01(progress01);
        progressScrollbar.value = 0f;
        progressScrollbar.size = p;
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
        if (progressScrollbar != null)
        {
            ApplyProgressScrollbarVisual(0f);
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

        float displayProgress = 0f;
        bool activatedNextScene = false;
        const float targetProgressEpsilon = 0.998f;

        while (!op.isDone)
        {
            if (_destroyed) return;

            // Unity 的 AsyncOperation.progress 載入完成前只會跑到 0.9，之後卡住直到 allowSceneActivation = true。
            bool loadStalledWaitingForActivation = op.progress >= 0.9f;
            float targetProgress = loadStalledWaitingForActivation ? 1f : Mathf.Clamp01(op.progress / 0.9f);

            float delta = Mathf.Max(progressCatchUpSpeed, 8f); // 最後拉到 100% 時保底不要太慢
            if (!loadStalledWaitingForActivation)
            {
                delta = progressCatchUpSpeed;
            }

            displayProgress = Mathf.MoveTowards(displayProgress, targetProgress, Time.unscaledDeltaTime * delta);

            if (progressScrollbar != null)
            {
                ApplyProgressScrollbarVisual(displayProgress);
            }

            if (loadStalledWaitingForActivation
                && !activatedNextScene
                && displayProgress >= targetProgressEpsilon)
            {
                activatedNextScene = true;
                displayProgress = 1f;
                if (progressScrollbar != null)
                {
                    ApplyProgressScrollbarVisual(1f);
                }

                var framesToWait = Mathf.Max(framesToShowFullBarBeforeActivate, 0);
                while (framesToWait-- > 0)
                {
                    await UniTask.Yield();
                    if (_destroyed) return;
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
