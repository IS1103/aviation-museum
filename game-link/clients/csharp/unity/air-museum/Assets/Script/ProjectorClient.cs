// ProjectorClient.cs - 航空館投影端：連線、認證、訂閱 OnPlayer/OnState/OnError，並可送 SendState。
// 使用方式見 doc/AirMuseumUsage.md
using System;
using Gate;
using GameLink.Libs.Client;
using UnityEngine;
using UnityEngine.UI;

namespace AirMuseum
{
    public class ProjectorClient : MonoBehaviour
    {
        [Header("連線設定")]
        [Tooltip("服務端路徑為 /ws。若服務在 WSL、Unity 在 Windows，localhost 可能連不到，請在 WSL 執行 hostname -I 取得 IP，改填 ws://<IP>:8770/ws")]
        [SerializeField] private string wsUrl = "ws://localhost:8770/ws";
        [Tooltip("若服務在 WSL，同上改用 WSL IP，例如 http://172.x.x.x:8771")]
        [SerializeField] private string httpBaseUrl = "http://localhost:8771";

        [Header("認證（投影端用 device=projector）")]
        [SerializeField] private string authToken = "your-jwt-or-key";

        [Header("目前遊戲階段（可透過程式或 UI 更新後呼叫 SendCurrentState）")]
        [SerializeField] private GamePhase currentPhase = GamePhase.WaitingEntry;

        [Header("UI（選填）")]
        [Tooltip("認證成功後會顯示 UID")]
        [SerializeField] private Text uidText;

        private bool _destroyed;

        private void Awake()
        {
            var svc = AirMuseumService.Instance;

            svc.OnPlayer += OnPlayerMessage;
            svc.OnState += OnStateMessage;
            svc.OnError += OnErrorMessage;
        }

        private void OnDestroy()
        {
            _destroyed = true;
            var svc = AirMuseumService.Instance;
            svc.OnPlayer -= OnPlayerMessage;
            svc.OnState -= OnStateMessage;
            svc.OnError -= OnErrorMessage;
        }

        private async void Start()
        {
            var svc = AirMuseumService.Instance;
            var runner = GetComponent<GameLinkClientRunner>() ?? gameObject.AddComponent<GameLinkClientRunner>();

            await svc.ConnectAsync(wsUrl, httpBaseUrl, runner);
            if (_destroyed) return;
            if (!svc.IsConnected)
                return;

            var payload = new ValidateReq
            {
                Token = authToken,
                GateSid = "",
                Device = "projector"
            };
            var resp = await svc.AuthAsync(payload);
            if (_destroyed) return;
            if (resp == null)
            {
                Debug.LogError("[AirMuseum] 認證失敗");
                return;
            }

            // resp.Uid 為服務端指派的 uid，resp.Msg 非空時表示錯誤訊息（已經 OnError 通知）
            Debug.Log($"[投影端] 認證成功 uid={resp.Uid}");
            if (uidText != null)
                uidText.text = "UID: " + resp.Uid;
            // 認證成功後可送一次目前狀態（可選）
            SendCurrentState();
        }

        /// <summary>收到玩家操作（投影端主要訂閱此事件）</summary>
        private void OnPlayerMessage(PlayerInput p)
        {
            if (_destroyed) return;

            switch (p.Action)
            {
                case Action.Entry:
                    Debug.Log($"[投影端] 玩家入桌 uid={p.Uid}");
                    // 可依 p.Uid 自行載入顯示資料（例如其他後端或本機快取）
                    break;
                case Action.Leave:
                    Debug.Log($"[投影端] 玩家離桌 uid={p.Uid}");
                    break;
                case Action.Input:
                    Debug.Log($"[投影端] 輸入 uid={p.Uid} axis=({p.AxisX}, {p.AxisY}) seq={p.Seq}");
                    break;
                default:
                    break;
            }
        }

        /// <summary>收到遊戲狀態（Server 廣播；投影端若需同步可訂閱）</summary>
        private void OnStateMessage(GameState s)
        {
            if (_destroyed) return;

            Debug.Log($"[投影端] 遊戲狀態 phase={s.State} uids={string.Join(",", s.Uids)}");
        }

        /// <summary>任何錯誤（連線、認證、推送錯誤）</summary>
        private void OnErrorMessage(string msg)
        {
            if (_destroyed) return;

            Debug.LogError("[AirMuseum] " + msg);
        }

        /// <summary>將目前設定的階段送給 Server（可由 UI 按鈕或程式呼叫）</summary>
        public void SendCurrentState()
        {
            if (!AirMuseumService.Instance.IsConnected) return;

            var state = new GameState { State = currentPhase };
            AirMuseumService.Instance.SendState(state);

            
        }

        /// <summary>設定階段並送出（方便從 Inspector 或程式切換）</summary>
        public void SetPhaseAndSend(GamePhase phase)
        {
            currentPhase = phase;
            SendCurrentState();
        }
    }
}
