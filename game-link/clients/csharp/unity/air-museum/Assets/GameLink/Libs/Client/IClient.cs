// IClient.cs - 客戶端介面定義（對應 Cocos game-link-cocos-sdk/libs/client）
using System;
using Google.Protobuf;

namespace GameLink.Libs.Client
{
    /// <summary>待處理請求（用於 request/response）</summary>
    public class PendingRequest<T> where T : IMessage
    {
        public Action<T> Resolve;
        public Action<Exception> Reject;
        public object AbortToken; // CancellationTokenSource or similar
    }

    /// <summary>客戶端介面：連線、request/notify/trigger/fetch、推送訂閱</summary>
    public interface IClient
    {
        string Url { get; }
        bool IsOpen { get; }
        string ConnType { get; }

        void OnReceive(byte[] data);

        void Send(string route, uint reqId, byte[] normalPack, Action<Exception> reject);

        (System.Threading.Tasks.Task<TRes> Promise, object AbortToken) Request<TReq, TRes>(
            string route, TReq payload, int timeoutMs = 15000)
            where TReq : IMessage where TRes : IMessage;

        (System.Threading.Tasks.Task<TRes> Promise, object AbortToken) Fetch<TRes>(
            string route, int timeoutMs = 15000) where TRes : IMessage;

        void Notify<TReq>(string route, TReq payload) where TReq : IMessage;
        void Trigger(string route);

        void OnNotify<T>(string route, Action<T> handler) where T : IMessage;
        void OffNotify<T>(string route, Action<T> handler) where T : IMessage;
        void OnNotifyErr(Action<string, string> handler);
        void OnClose(Action<ushort, string> handler);
        void OffClose(Action<ushort, string> handler);

        void StopScan();
        void AbortAll(string reason = null);
        uint GetNextReqId();

        System.Threading.Tasks.Task ConnectAsync();
        bool IsConnected();
        System.Threading.Tasks.Task WaitForConnectionAsync(); // 等同 await OpenPromise（連線完成）

        void OnReconnect(Action handler);
        void OnReconnectFailed(Action handler);
        void OnBeforeReconnect(Func<System.Threading.Tasks.Task> handler);
    }
}
