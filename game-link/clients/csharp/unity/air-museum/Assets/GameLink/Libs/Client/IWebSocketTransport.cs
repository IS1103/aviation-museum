// IWebSocketTransport.cs - WebSocket 傳輸介面（可替換為 NativeWebSocket、WebSocketSharp 等）
using System;

namespace GameLink.Libs.Client
{
    /// <summary>WebSocket 傳輸層，由外部注入實作（例如 NativeWebSocket）</summary>
    public interface IWebSocketTransport
    {
        bool IsOpen { get; }
        void Connect(string url, Action onOpen, Action<byte[]> onMessage, Action<ushort, string> onClose, Action<Exception> onError);
        void Send(byte[] data);
        void Close();
    }
}
