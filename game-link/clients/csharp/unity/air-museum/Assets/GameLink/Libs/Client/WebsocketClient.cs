// WebsocketClient.cs - WebSocket 客戶端（對應 Cocos WebsocketClient）
using System;
using System.Threading.Tasks;

namespace GameLink.Libs.Client
{
    public class WebsocketClient : ClientBase
    {
        private const int MaxReconnectAttempts = 3;

        private IWebSocketTransport _transport;
        private bool _shouldReconnect = true;
        private bool _hasConnectedOnce;
        private int _retry = MaxReconnectAttempts;
        private TaskCompletionSource<bool> _connectTcs;

        /// <param name="url">WebSocket URL</param>
        /// <param name="transport">傳輸層實作；若為 null 則在 Connect 時自動使用 DefaultWebSocketTransport（若可用）</param>
        /// <param name="mainThreadRunner">主線程派發器（如 GameLinkClientRunner）；null 時使用全域 GameLinkMainThreadDispatcher</param>
        /// <param name="autoReconnect">是否自動重連</param>
        public WebsocketClient(string url, IWebSocketTransport transport = null, IMainThreadRunner mainThreadRunner = null, bool autoReconnect = true)
        {
            Url = url ?? "";
            ConnType = "WS";
            _shouldReconnect = autoReconnect;
            _transport = transport;
            SetMainThreadRunner(mainThreadRunner);
        }

        public IWebSocketTransport Transport
        {
            get => _transport;
            set => _transport = value;
        }

        public override async Task ConnectAsync()
        {
            if (_transport == null)
                _transport = new DefaultWebSocketTransport();

            _connectTcs = new TaskCompletionSource<bool>();

            if (!_hasConnectedOnce && _retry != MaxReconnectAttempts)
                _retry = MaxReconnectAttempts;

            _transport.Connect(
                Url,
                onOpen: () =>
                {
                    DispatchToMainThread(() =>
                    {
                        var isReconnect = _hasConnectedOnce;
                        _hasConnectedOnce = true;
                        _retry = MaxReconnectAttempts;
                        IsOpen = true;
                        _openResolve?.Invoke();
                        _openResolve = null;
                        _connectTcs.TrySetResult(true);
                        if (isReconnect)
                            EmitReconnect();
                    });
                },
                onMessage: data =>
                {
                    var copy = data;
                    DispatchToMainThread(() => OnReceive(copy));
                },
                onClose: (code, reason) =>
                {
                    var c = code;
                    var r = reason ?? "";
                    DispatchToMainThread(() =>
                    {
                        IsOpen = false;
                        AbortAll("Connection closed");
                        EmitClose(c, r);
                        if (_shouldReconnect && _retry > 0)
                        {
                            _retry--;
                            Task.Run(async () =>
                            {
                                await Task.Delay(ProtoSchemaManager.RetryIntervalMs);
                                try
                                {
                                    await EmitBeforeReconnect();
                                }
                                catch (Exception ex)
                                {
                                    UnityEngine.Debug.LogError($"[{ConnType}] BeforeReconnect failed: " + ex.Message);
                                    _retry = 0;
                                    EmitReconnectFailed();
                                    _shouldReconnect = false;
                                    return;
                                }
                                try
                                {
                                    await ConnectAsync();
                                }
                                catch (Exception ex)
                                {
                                    UnityEngine.Debug.LogError($"[{ConnType}] Reconnect failed: " + ex.Message);
                                }
                            });
                        }
                        else if (_shouldReconnect && _retry <= 0)
                        {
                            EmitReconnectFailed();
                            _shouldReconnect = false;
                        }
                    });
                },
                onError: ex =>
                {
                    var e = ex;
                    DispatchToMainThread(() => _connectTcs.TrySetException(e));
                }
            );

            await _connectTcs.Task;
        }

        public override void OnReceive(byte[] data)
        {
            try
            {
                ProcessReceivedData(data);
            }
            catch (Exception ex)
            {
                UnityEngine.Debug.LogException(ex);
                foreach (var kv in Pending)
                {
                    try { kv.Value.Reject(ex); kv.Value.Cts?.Cancel(); } catch { }
                    Pending.Clear();
                }
            }
        }

        public override void Send(string route, uint reqId, byte[] normalPack, Action<Exception> reject)
        {
            if (_transport == null || !_transport.IsOpen)
            {
                reject(new InvalidOperationException($"[{ConnType}] Not connected"));
                return;
            }
            try
            {
                _transport.Send(normalPack);
            }
            catch (Exception ex)
            {
                UnityEngine.Debug.LogException(ex);
                reject(ex);
            }
        }

        /// <summary>關閉連線並停止重連</summary>
        public override void StopScan()
        {
            _shouldReconnect = false;
            _hasConnectedOnce = false;
            _retry = 0;
            AbortAll("Client closed");
            _transport?.Close();
            _transport = null;
        }

        public override bool IsConnected() => _transport != null && _transport.IsOpen;
    }
}
