// AirMuseumService.cs - 航空館連線與 API 單例
// 使用方式：ConnectAsync(wsUrl)；錯誤統一經 OnError(msg)；事件僅支援 +=。
// auth/validate 以 ValidateReq.Device 區分終端（見 doc）；選填 token 僅 uid= 續連或 register&…。
using System;
using Cysharp.Threading.Tasks;
using Gate;
using AirMuseum;
using GameLink.Libs.Client;
using UnityEngine;

namespace AirMuseum
{
    /// <summary>
    /// 航空館服務單例：WebSocket 連線、認證（含首登註冊）、player/state 發送與訂閱。
    /// 所有錯誤不拋出，統一經 OnError(msg) 通知；OnError 必在主線程觸發。
    /// </summary>
    public class AirMuseumService
    {
        private static AirMuseumService _instance;

        public static AirMuseumService Instance => _instance ?? (_instance = new AirMuseumService());

        private WebsocketClient _client;

        /// <summary>收到 Server 推送的 PlayerInput（air_museum/player）</summary>
        public event Action<PlayerInput> OnPlayer;

        /// <summary>收到 Server 推送的 GameState（air_museum/state）</summary>
        public event Action<GameState> OnState;

        /// <summary>收到 notify air_museum/add_player（推送玩家資料，例如手機端 SAVE_APPEARANCE 後）</summary>
        public event Action<AddPlayerNotify> OnAddPlayer;

        /// <summary>任何錯誤（連線、認證、WS 錯誤推送）僅傳錯誤訊息字串</summary>
        public event Action<string> OnError;

        private AirMuseumService() { }

        /// <summary>統一錯誤出口：確保在主線程觸發 OnError。</summary>
        private void EmitError(string msg)
        {
            if (string.IsNullOrEmpty(msg)) return;
            GameLinkMainThreadDispatcher.Enqueue(() => OnError?.Invoke(msg));
        }

        /// <summary>
        /// 建立 WebSocket 連線。wsUrl 必填。
        /// mainThreadRunner 若傳入（如 GameLinkClientRunner），封包與回調會由該 MonoBehaviour 在主線程處理；null 則用全域 Dispatcher。
        /// 連線失敗不拋錯，改經 OnError(msg) 通知。
        /// </summary>
        public async UniTask ConnectAsync(string wsUrl, IMainThreadRunner mainThreadRunner = null)
        {
            if (string.IsNullOrEmpty(wsUrl))
            {
                EmitError("ConnectAsync 需要 wsUrl");
                return;
            }

            try
            {
                _client = new WebsocketClient(wsUrl, new DefaultWebSocketTransport(), mainThreadRunner, true);
                RegisterClientHandlers();
                await _client.ConnectAsync();
            }
            catch (Exception ex)
            {
                var msg = ex.Message ?? "連線失敗";
                if (ex.InnerException != null)
                    msg += " (" + ex.InnerException.Message + ")";
                EmitError($"連線失敗 ({wsUrl}): {msg}");
            }
        }

        private void RegisterClientHandlers()
        {
            if (_client == null) return;

            _client.OnNotify<PlayerInput>("air_museum/player", p => OnPlayer?.Invoke(p));
            _client.OnNotify<GameState>("air_museum/state", s => OnState?.Invoke(s));
            _client.OnNotify<AddPlayerNotify>("air_museum/add_player", p => OnAddPlayer?.Invoke(p));
            _client.OnNotify<ErrorNotify>("air_museum/error", e => OnError?.Invoke(e.Msg ?? ""));
            _client.OnNotifyErr((route, msg) => OnError?.Invoke(msg ?? ""));
        }

        /// <summary>認證。成功回傳 ValidateResp（含 Uid、Msg），失敗回傳 null 並經 OnError(msg) 通知。</summary>
        public async UniTask<ValidateResp> AuthAsync(ValidateReq payload)
        {
            if (_client == null)
            {
                EmitError("請先呼叫 ConnectAsync");
                return null;
            }

            try
            {
                var (promise, _) = _client.Request<ValidateReq, ValidateResp>("auth/validate", payload);
                Debug.Log($"[AirMuseum] 認證請求: {payload.Token} {payload.GateSid} {payload.Device}");
                var resp = await promise;
                Debug.Log($"[AirMuseum] 認證回應: {resp?.Uid} {resp?.Msg}");
                if (resp == null)
                {
                    EmitError("認證無回應");
                    return null;
                }
                if (!string.IsNullOrEmpty(resp.Msg))
                    EmitError(resp.Msg);
                return resp;
            }
            catch (TimeoutException)
            {
                Debug.LogWarning("[AirMuseum] 認證逾時（未在時限內收到伺服器回應）");
                return null;
            }
            catch (OperationCanceledException)
            {
                // 外部取消（如場景卸載、CancellationToken），不當成錯誤通知
                Debug.Log("[AirMuseum] 認證已取消");
                return null;
            }
            catch (Exception ex)
            {
                var msg = ex.Message ?? "認證失敗";
                if (ex.InnerException != null)
                    msg += " (" + ex.InnerException.Message + ")";
                Debug.LogWarning($"[AirMuseum] 認證回應（失敗）: {msg}");
                EmitError(msg);
                return null;
            }
        }

        /// <summary>
        /// 組 auth/validate 的註冊 token：register&name=…&age=…&sex=…（與 ValidateReq.Device 併用，勿在 token 內帶 device）。
        /// </summary>
        public static string BuildRegisterToken(string name, int age, int sex)
        {
            var n = string.IsNullOrEmpty(name) ? "" : UnityEngine.Networking.UnityWebRequest.EscapeURL(name);
            return $"register&name={n}&age={age}&sex={sex}";
        }

        /// <summary>續連 token：僅 uid=&lt;n&gt;（身分一律由 ValidateReq.Device 指定）。</summary>
        public static string BuildResumeToken(uint uid)
        {
            return $"uid={uid}";
        }

        /// <summary>發送玩家操作（air_museum/player）。</summary>
        public void SendPlayer(PlayerInput payload)
        {
            if (_client == null) return;
            _client.Notify("air_museum/player", payload);
        }

        /// <summary>發送遊戲狀態（air_museum/state），供投影端使用。</summary>
        public void SendState(GameState payload)
        {
            if (_client == null) return;
            _client.Notify("air_museum/state", payload);
        }

        /// <summary>是否已連線</summary>
        public bool IsConnected => _client != null && _client.IsConnected();
    }
}
