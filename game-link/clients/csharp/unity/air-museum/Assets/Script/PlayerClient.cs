// PlayerClient.cs - 航空館手機端／玩家端：連線、認證、訂閱 OnState/OnError，送 Entry/Leave/Input。
// 使用方式見 doc/AirMuseumUsage.md
using System;
using Gate;
using GameLink.Libs.Client;
using UnityEngine;
using UnityEngine.UI;

namespace AirMuseum
{
    /// <summary>
    /// 手機端 Client：device=player 認證後可入桌、離桌、送遊戲輸入；
    /// 主要訂閱 OnState 收遊戲階段與房內玩家，錯誤經 OnError。
    /// </summary>
    public class PlayerClient : MonoBehaviour
    {
        [Header("連線設定")]
        [Tooltip("服務端路徑為 /ws。手機與服務同網段時可用本機 IP，例如 ws://192.168.1.100:8770/ws")]
        [SerializeField] private string wsUrl = "ws://localhost:8770/ws";
        [Tooltip("同上，HTTP 基底，例如 http://192.168.1.100:8771")]
        [SerializeField] private string httpBaseUrl = "http://localhost:8771";

        [Header("認證（玩家端用 device=player；token 可含 key=uid）")]
        [SerializeField] private string authToken = "key=1&device=player";

        [Header("UI（選填）")]
        [Tooltip("認證成功後會顯示 UID")]
        [SerializeField] private Text uidText;
        [Tooltip("連線／遊戲階段等狀態文字")]
        [SerializeField] private Text statusText;

        private bool _destroyed;
        private uint _myUid;
        private uint _inputSeq;

        private void Awake()
        {
            var svc = AirMuseumService.Instance;
            svc.OnState += OnStateMessage;
            svc.OnError += OnErrorMessage;
        }

        private void OnDestroy()
        {
            _destroyed = true;
            var svc = AirMuseumService.Instance;
            svc.OnState -= OnStateMessage;
            svc.OnError -= OnErrorMessage;
        }

        private async void Start()
        {
            SetStatus("連線中…");
            var svc = AirMuseumService.Instance;
            var runner = GetComponent<GameLinkClientRunner>() ?? gameObject.AddComponent<GameLinkClientRunner>();

            await svc.ConnectAsync(wsUrl, httpBaseUrl, runner);
            if (_destroyed) return;
            if (!svc.IsConnected)
            {
                SetStatus("連線失敗");
                return;
            }

            SetStatus("認證中…");
            var payload = new ValidateReq
            {
                Token = authToken,
                GateSid = "",
                Device = "player"
            };
            var resp = await svc.AuthAsync(payload);
            if (_destroyed) return;
            if (resp == null)
            {
                SetStatus("認證失敗");
                return;
            }

            _myUid = resp.Uid;
            Debug.Log($"[手機端] 認證成功 uid={_myUid}");
            if (uidText != null)
                uidText.text = "UID: " + _myUid;
            SetStatus("已連線 (入桌請按入桌)");
        }

        private void SetStatus(string msg)
        {
            if (statusText != null)
                statusText.text = msg;
        }

        /// <summary>收到遊戲狀態（Server 廣播；手機端主要訂閱此事件）</summary>
        private void OnStateMessage(GameState s)
        {
            if (_destroyed) return;

            var phaseStr = s.State.ToString();
            var uidsStr = s.Uids != null && s.Uids.Count > 0 ? string.Join(",", s.Uids) : "-";
            Debug.Log($"[手機端] 遊戲狀態 phase={phaseStr} uids=[{uidsStr}]");
            SetStatus($"階段: {phaseStr} | 房內: {uidsStr}");
        }

        private void OnErrorMessage(string msg)
        {
            if (_destroyed) return;

            Debug.LogError("[AirMuseum] " + msg);
            SetStatus("錯誤: " + msg);
        }

        /// <summary>入桌（可由 UI 按鈕呼叫）</summary>
        public void SendEntry()
        {
            if (!AirMuseumService.Instance.IsConnected) return;

            AirMuseumService.Instance.SendPlayer(new PlayerInput { Action = Action.Entry });
            SetStatus("已送出入桌");
        }

        /// <summary>離桌（可由 UI 按鈕呼叫）</summary>
        public void SendLeave()
        {
            if (!AirMuseumService.Instance.IsConnected) return;

            AirMuseumService.Instance.SendPlayer(new PlayerInput { Action = Action.Leave });
            SetStatus("已送出離桌");
        }

        /// <summary>送遊戲輸入（可由搖桿或按鈕呼叫；axis 建議 -1～1）</summary>
        public void SendInput(float axisX, float axisY)
        {
            if (!AirMuseumService.Instance.IsConnected) return;

            _inputSeq++;
            AirMuseumService.Instance.SendPlayer(new PlayerInput
            {
                Action = Action.Input,
                AxisX = axisX,
                AxisY = axisY,
                Seq = _inputSeq
            });
        }

        /// <summary>目前認證後的 UID（認證前為 0）</summary>
        public uint MyUid => _myUid;
    }
}
