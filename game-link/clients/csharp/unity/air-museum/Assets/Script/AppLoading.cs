using System.Collections;
using System.Threading.Tasks;
using AirMuseum;
using GameLink.Libs.Client;
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
    [Tooltip("HTTP 基底（目前服務保留用），例如 http://192.168.1.100:8771")]
    [SerializeField] private string httpBaseUrl = "http://localhost:8771";

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

        StartCoroutine(BootFlow());
    }

    private IEnumerator BootFlow()
    {
        var connectTask = ConnectAirMuseumAsync();
        while (!connectTask.IsCompleted)
        {
            yield return null;
        }
        if (_destroyed) yield break;

        if (!AirMuseumService.Instance.IsConnected)
        {
            Debug.LogError($"[AppLoading] 連線失敗，停止載入流程 (wsUrl={wsUrl})");
            yield break;
        }

        Debug.Log($"[AppLoading] 連線成功，開始載入場景 (wsUrl={wsUrl}, next={nextSceneName})");
        yield return LoadNextSceneAsync();
    }

    private async Task ConnectAirMuseumAsync()
    {
        var svc = AirMuseumService.Instance;
        if (svc.IsConnected) return;

        var runner = GetComponent<GameLinkClientRunner>() ?? gameObject.AddComponent<GameLinkClientRunner>();
        await svc.ConnectAsync(wsUrl, httpBaseUrl, runner);
    }

    private IEnumerator LoadNextSceneAsync()
    {
        AsyncOperation op = SceneManager.LoadSceneAsync(nextSceneName);
        op.allowSceneActivation = false;

        while (!op.isDone)
        {
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

            yield return null;
        }
    }

    private void OnAirMuseumError(string msg)
    {
        if (_destroyed) return;
        Debug.LogError("[AppLoading] AirMuseum 錯誤: " + msg);
    }
}
