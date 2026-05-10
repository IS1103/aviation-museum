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
    [DefaultExecutionOrder(-100)]
    public class AppPlayerClient : MonoBehaviour
    {
        [Header("連線設定")]
        [Tooltip("服務端路徑為 /ws。手機與服務同網段時可用本機 IP，例如 ws://192.168.1.100:8770/ws")]
        [SerializeField] private string wsUrl = "ws://localhost:8770/ws";

        [Header("認證")]
        [Tooltip("ValidateReq.Device。玩家 app 用 player；其它見 aviation-museum 裝置對照表。")]
        [SerializeField] private string authDevice = "player";

        [Tooltip("除錯覆寫。留空則每次開啟皆新玩家（先 DeleteAll 再認證，token 為空）。")]
        [SerializeField] private string authToken = "";

        // [Header("UI（選填）")]
        // [Tooltip("認證成功後會顯示 UID")]
        // [SerializeField] private Text uidText;
        [Tooltip("連線／遊戲階段等狀態文字")]
        [SerializeField] private Text statusText;



        private bool _destroyed;
        private uint _myUid;
        private uint _inputSeq;

        private void Awake()
        {
            // 手機 app 每次開啟視為新玩家：清掉舊 uid／註冊與裝扮快取，避免續連上一局身分。
            PlayerPrefs.DeleteAll();

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

            await svc.ConnectAsync(wsUrl, runner);
            if (_destroyed) return;
            if (!svc.IsConnected)
            {
                SetStatus("連線失敗");
                return;
            }

            SetStatus("認證中…");
            // 每次冷啟已 DeleteAll，此處不讀舊 uid；除錯請在 Inspector 填 authToken（例如 uid=123）。
            var token = authToken ?? "";

            var payload = new ValidateReq
            {
                Token = token ?? "",
                GateSid = "",
                Device = authDevice
            };
            var resp = await svc.AuthAsync(payload);
            if (_destroyed) return;
            if (resp == null)
            {
                SetStatus("認證失敗");
                return;
            }

            _myUid = resp.Uid;
            PlayerPrefs.SetInt("air_museum_uid", (int)_myUid);
            PlayerPrefs.Save();
            Debug.Log($"[手機端] 認證成功 uid={_myUid}（已寫入 PlayerPrefs）");
            // if (uidText != null)
            //     uidText.text = "UID: " + _myUid;
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

        /// <summary>
        /// 自 PlayerPrefs（<c>air_museum_*</c>）記錄目前註冊／裝扮，並以 <c>air_museum/player</c> 寫回伺服器 DB。
        /// </summary>
        public void ConfirmClothing()
        {
            int ageInt = Mathf.Clamp(PlayerPrefs.GetInt("air_museum_age", 0), 0, 150);
            Debug.Log(
                "【確認裝扮】PlayerPrefs\n" +
                "使用者 ID： " + PlayerPrefs.GetInt("air_museum_uid", 0) + "\n" +
                "姓名： " + PlayerPrefs.GetString("air_museum_name", "") + "\n" +
                "年齡： " + ageInt + "\n" +
                "性別： " + PlayerPrefs.GetInt("air_museum_sex", -1) + "\n" +
                "眼睛索引： " + PlayerPrefs.GetInt("air_museum_eyes_index", 0) + "\n" +
                "眉毛索引： " + PlayerPrefs.GetInt("air_museum_eyebrow_index", 0) + "\n" +
                "嘴巴索引： " + PlayerPrefs.GetInt("air_museum_mouth_index", 0) + "\n" +
                "眼鏡索引： " + PlayerPrefs.GetInt("air_museum_glasses_index", -1) + "\n" +
                "頭盔索引： " + PlayerPrefs.GetInt("air_museum_helmet_index", 0));

            if (!AirMuseumService.Instance.IsConnected)
            {
                Debug.LogWarning("[手機端] 確認裝扮：未連線，略過同步伺服器");
                return;
            }

            AirMuseumService.Instance.SendPlayer(new PlayerInput
            {
                Action = Action.SaveAppearance,
                Name = PlayerPrefs.GetString("air_museum_name", ""),
                Age = (uint)ageInt,
                Sex = PlayerPrefs.GetInt("air_museum_sex", -1),
                AvatarEyes = PlayerPrefs.GetInt("air_museum_eyes_index", 0),
                AvatarEyebrow = PlayerPrefs.GetInt("air_museum_eyebrow_index", 0),
                AvatarMouth = PlayerPrefs.GetInt("air_museum_mouth_index", 0),
                AvatarGlasses = PlayerPrefs.GetInt("air_museum_glasses_index", -1),
                AvatarHelmet = PlayerPrefs.GetInt("air_museum_helmet_index", 0),
            });
        }
    }
}
