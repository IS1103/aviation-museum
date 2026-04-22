// DefaultWebSocketTransport.cs - 使用 System.Net.WebSockets 的預設實作（WebGL 等平台請改用 NativeWebSocket 等）
// 說明：System.Net.WebSockets 的 API 本身回傳 .NET 原生 Task，這是 BCL 的限制；
//      本專案其餘程式碼一律改用 UniTask，這裡透過 .AsUniTask() 把 BCL Task 橋接到 UniTask。
using System;
using System.Net.WebSockets;
using System.Threading;
using Cysharp.Threading.Tasks;

namespace GameLink.Libs.Client
{
    public class DefaultWebSocketTransport : IWebSocketTransport
    {
        private ClientWebSocket _ws;
        private CancellationTokenSource _cts;
        private Action _onOpen;
        private Action<byte[]> _onMessage;
        private Action<ushort, string> _onClose;
        private Action<Exception> _onError;

        public bool IsOpen => _ws?.State == WebSocketState.Open;

        public void Connect(string url, Action onOpen, Action<byte[]> onMessage, Action<ushort, string> onClose, Action<Exception> onError)
        {
            _onOpen = onOpen;
            _onMessage = onMessage;
            _onClose = onClose;
            _onError = onError;
            _cts = new CancellationTokenSource();
            RunAsync(url).Forget();
        }

        private async UniTaskVoid RunAsync(string url)
        {
            try
            {
                _ws = new ClientWebSocket();
                await _ws.ConnectAsync(new Uri(url), _cts.Token).AsUniTask();
                _onOpen?.Invoke();
                var buffer = new byte[1024 * 64];
                while (_ws.State == WebSocketState.Open && !_cts.Token.IsCancellationRequested)
                {
                    var result = await _ws.ReceiveAsync(new ArraySegment<byte>(buffer), _cts.Token).AsUniTask();
                    if (result.MessageType == WebSocketMessageType.Close)
                    {
                        _onClose?.Invoke((ushort)result.CloseStatus.Value, result.CloseStatusDescription ?? "");
                        break;
                    }
                    if (result.Count > 0)
                    {
                        var copy = new byte[result.Count];
                        Array.Copy(buffer, 0, copy, 0, result.Count);
                        _onMessage?.Invoke(copy);
                    }
                }
            }
            catch (Exception ex)
            {
                // 正常關閉（Close() 取消 Token）時會拋 OperationCanceledException / TaskCanceledException（後者繼承自前者），不當成錯誤通知
                if (ex is OperationCanceledException) return;
                if (ex.InnerException is OperationCanceledException) return;
                _onError?.Invoke(ex);
            }
            finally
            {
                var code = (ushort)(_ws?.CloseStatus ?? 0);
                var reason = _ws?.CloseStatusDescription ?? "";
                try { _ws?.Dispose(); } catch { }
                _ws = null;
                _onClose?.Invoke(code, reason);
            }
        }

        public void Send(byte[] data)
        {
            if (_ws?.State != WebSocketState.Open)
                throw new InvalidOperationException("WebSocket not open");
            // BCL SendAsync 回傳 Task，這裡是同步 API 所以直接 GetResult 阻塞等待
            _ws.SendAsync(new ArraySegment<byte>(data), WebSocketMessageType.Binary, true, _cts.Token).GetAwaiter().GetResult();
        }

        public void Close()
        {
            _cts?.Cancel();
            try
            {
                _ws?.CloseAsync(WebSocketCloseStatus.NormalClosure, "", CancellationToken.None).GetAwaiter().GetResult();
            }
            catch { }
            _ws?.Dispose();
            _ws = null;
        }
    }
}
