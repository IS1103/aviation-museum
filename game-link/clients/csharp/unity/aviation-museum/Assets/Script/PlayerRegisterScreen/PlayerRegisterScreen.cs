// 玩家註冊用大螢：連線並以 device=playerRegisterScreen 認證；連線 uid 固定為 2（單館單線）。
// 流程見 doc/AirMuseumUsage.md 與 game-link/services/air-museum/doc/流程與時序.md
using Gate;
using GameLink.Libs.Client;
using UnityEngine;
using UnityEngine.UI;

namespace AirMuseum
{
    public class PlayerRegisterScreen : MonoBehaviour
    {
        [Header("連線設定")]
        [SerializeField]
        private string wsUrl = "ws://localhost:8770/ws";

        [Header("認證")]
        [Tooltip("固定為玩家註冊螢幕；請勿改用 player")]
        [SerializeField]
        private string authDevice = "playerRegisterScreen";

        [Tooltip("固定終端不使用 token")]
        [SerializeField]
        private string authToken = "";

        [Header("UI（選填）")]
        [SerializeField]
        private Text statusText;

        private bool _destroyed;

        private void Awake()
        {
            var svc = AirMuseumService.Instance;
            svc.OnError += OnErrorMessage;
            svc.OnAddPlayer += AddPlayer;
        }

        private void OnDestroy()
        {
            _destroyed = true;
            var svc = AirMuseumService.Instance;
            svc.OnAddPlayer -= AddPlayer;
            svc.OnError -= OnErrorMessage;
        }

        private async void Start()
        {
            SetStatus("連線中…");
            var svc = AirMuseumService.Instance;
            var runner = GetComponent<GameLinkClientRunner>() ?? gameObject.AddComponent<GameLinkClientRunner>();

            await svc.ConnectAsync(wsUrl, runner);
            if (_destroyed)
                return;
            if (!svc.IsConnected)
            {
                SetStatus("連線失敗");
                return;
            }

            SetStatus("認證中…");
            var payload = new ValidateReq
            {
                Token = authToken ?? "",
                GateSid = "",
                Device = authDevice,
            };

            var resp = await svc.AuthAsync(payload);
            if (_destroyed)
                return;
            if (resp == null)
            {
                SetStatus("認證失敗");
                return;
            }

            Debug.Log($"[PlayerRegisterScreen] 認證成功 uid={resp.Uid}（預期為 2）");
            SetStatus($"已連線 uid={resp.Uid}");
        }

        /// <summary>
        /// Server 經 notify <c>air_museum/add_player</c> 推送：一名玩家在 Postgres <c>player</c> 表之完整對應欄位。
        /// 目前於手機端 <see cref="Action.SaveAppearance"/> 成功並寫入 DB 後觸發。
        /// </summary>
        public void AddPlayer(AddPlayerNotify p)
        {
            if (_destroyed || p == null)
                return;

            var msg =
                $"[PlayerRegisterScreen] add_player uid={p.Uid} name={p.Name} age={p.Age} sex={p.Sex} " +
                $"mission={p.Mission} score={p.GameScore}/{p.LandingScore} rank={p.Ranking} " +
                $"createdUnix={p.CreattimeUnixSeconds}";
            Debug.Log(msg);
            SetStatus($"玩家 uid={p.Uid} {p.Name}");

            OnPlayerDataFromServer(p);
        }

        /// <summary>供子類或本場景延伸：列表同步、詳情 UI 等。</summary>
        protected virtual void OnPlayerDataFromServer(AddPlayerNotify _)
        {
        }

        private void SetStatus(string msg)
        {
            if (statusText != null)
                statusText.text = msg;
        }

        private void OnErrorMessage(string msg)
        {
            if (_destroyed)
                return;
            Debug.LogError("[AirMuseum][PlayerRegisterScreen] " + msg);
            SetStatus("錯誤: " + msg);
        }
    }
}
