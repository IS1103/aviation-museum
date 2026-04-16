// ClientBase.cs - 客戶端抽象基類（對應 Cocos ClientBase）
using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using Gate;
using UnityEngine;

namespace GameLink.Libs.Client
{
    public class PendingItem
    {
        public Action<IMessage> Resolve;
        public Action<Exception> Reject;
        public CancellationTokenSource Cts;
    }

    public abstract class ClientBase : IClient
    {
        protected Action _openResolve;
        protected readonly Dictionary<string, List<Delegate>> PushHandlers = new Dictionary<string, List<Delegate>>();
        protected readonly List<Action<string, string>> ErrorHandlers = new List<Action<string, string>>();
        protected readonly List<Action<ushort, string>> CloseHandlers = new List<Action<ushort, string>>();
        protected readonly List<Action> ReconnectHandlers = new List<Action>();
        protected readonly List<Action> ReconnectFailedHandlers = new List<Action>();
        protected readonly List<Func<Task>> BeforeReconnectHandlers = new List<Func<Task>>();

        protected readonly Dictionary<uint, PendingItem> Pending = new Dictionary<uint, PendingItem>();
        protected IMainThreadRunner MainThreadRunner { get; set; }

        public string Url { get; set; }
        public bool IsOpen { get; set; }
        public Task OpenPromise { get; private set; }
        public string ConnType { get; protected set; }
        protected uint ReqIdCounter = 1;
        private readonly HashSet<string> _loggedPushErrorKeys = new HashSet<string>();

        protected ClientBase()
        {
            Url = "";
            ConnType = "";
            var tcs = new TaskCompletionSource<bool>();
            OpenPromise = tcs.Task.ContinueWith(_ => { });
            _openResolve = () => tcs.TrySetResult(true);
        }

        /// <summary>設定主線程派發器（如 GameLinkClientRunner）。未設定時會使用全域 GameLinkMainThreadDispatcher。</summary>
        public void SetMainThreadRunner(IMainThreadRunner runner) => MainThreadRunner = runner;

        /// <summary>將 action 派發到主線程執行（使用注入的 Runner 或全域 Dispatcher）。</summary>
        protected void DispatchToMainThread(Action a) => (MainThreadRunner ?? (IMainThreadRunner)GameLinkMainThreadDispatcher.Instance)?.Enqueue(a);

        public abstract Task ConnectAsync();
        public abstract void OnReceive(byte[] data);
        public abstract void Send(string route, uint reqId, byte[] normalPack, Action<Exception> reject);

        /// <summary>解析收到的 Pack 並分派到 response 或 push handler（由 WebsocketClient 在 OnReceive 中呼叫）</summary>
        protected void ProcessReceivedData(byte[] data)
        {
            var (decoded, dataMessage) = ParsePack(data);
            if (decoded.PackType == 1)
                ResponseHandler(decoded, dataMessage);
            else if (decoded.PackType == 2)
                BroadcastHandler(decoded, dataMessage);
            else
                Debug.LogWarning($"[{ConnType}] Unknown packType: {decoded.PackType}");
        }

        protected (string svt, string method) ParseRoute(string route)
        {
            var parts = route.Split('/');
            if (parts.Length != 2 || string.IsNullOrEmpty(parts[0]) || string.IsNullOrEmpty(parts[1]))
                throw new ArgumentException($"Invalid route: {route}. Expected svt/method");
            return (parts[0], parts[1]);
        }

        protected string BuildRouteString(string svt, string method, string fallback = null)
        {
            if (!string.IsNullOrEmpty(svt) && !string.IsNullOrEmpty(method))
                return $"{svt}/{method}";
            return !string.IsNullOrEmpty(fallback) ? fallback : "unknown";
        }

        protected (Pack decoded, IMessage dataMessage) ParsePack(byte[] u8)
        {
            var pack = Pack.Parser.ParseFrom(u8);
            IMessage dataMessage = null;
            if (pack.Info != null && !string.IsNullOrEmpty(pack.Info.TypeUrl) && pack.Info.Value != null && pack.Info.Value.Length > 0)
            {
                ProtoSchemaManager.TryParse(pack.Info.TypeUrl, pack.Info.Value, out dataMessage);
            }
            return (pack, dataMessage);
        }

        protected void ResponseHandler(Pack decoded, IMessage dataMessage)
        {
            if (!Pending.TryGetValue(decoded.ReqId, out var item))
                return;
            Pending.Remove(decoded.ReqId);
            item.Cts?.Cancel();
            var routeStr = BuildRouteString(decoded.Svt, decoded.Method, null);
            if (!decoded.Success)
            {
                var errMsg = string.IsNullOrEmpty(decoded.Msg) ? "Request failed" : decoded.Msg;
                Debug.LogError($"🔴[{ConnType}][{decoded.ReqId}][◀][{routeStr}] {errMsg}");
                item.Reject(new Exception(errMsg));
                foreach (var h in ErrorHandlers) h(routeStr, errMsg);
                return;
            }
            if (routeStr != "gate/ping")
                Debug.Log($"🔵[{ConnType}][Response][{decoded.ReqId}][◀][{routeStr}] {(dataMessage != null ? dataMessage.ToString() : "null")}");
            try
            {
                item.Resolve(dataMessage);
            }
            catch (Exception ex)
            {
                item.Reject(ex);
            }
        }

        protected void BroadcastHandler(Pack decoded, IMessage dataMessage)
        {
            var routeStr = BuildRouteString(decoded.Svt, decoded.Method, null);
            if (!decoded.Success)
            {
                var errMsg = decoded.Msg ?? "";
                if (dataMessage is ErrorDetail ed)
                    errMsg = ed.Msg ?? errMsg;
                var key = $"{routeStr}|{errMsg}";
                if (!_loggedPushErrorKeys.Contains(key))
                {
                    _loggedPushErrorKeys.Add(key);
                    Debug.LogError($"🔴[{ConnType}][Push Error][{routeStr}] {errMsg}");
                }
                foreach (var h in ErrorHandlers) h(routeStr, errMsg);
                return;
            }
            if (dataMessage != null)
                Debug.Log($"🟡[{ConnType}][Push][◀][{routeStr}] {dataMessage}");
            if (!PushHandlers.TryGetValue(routeStr, out var list) || list.Count == 0)
                return;
            if (dataMessage == null)
                return;
            foreach (var h in list)
                try { h.DynamicInvoke(dataMessage); } catch (Exception ex) { Debug.LogException(ex); }
        }

        public (Task<TRes> Promise, object AbortToken) Request<TReq, TRes>(string route, TReq payload, int timeoutMs = 15000)
            where TReq : IMessage where TRes : IMessage
        {
            var (svt, method) = ParseRoute(route);
            var reqId = GetNextReqId();
            var pack = new Pack
            {
                PackType = 0,
                ReqId = reqId,
                Svt = svt,
                Method = method,
                Info = new Google.Protobuf.WellKnownTypes.Any
                {
                    TypeUrl = "type.googleapis.com/" + payload.Descriptor.FullName,
                    Value = ByteString.CopyFrom(payload.ToByteArray())
                }
            };
            var bin = pack.ToByteArray();
            var cts = new CancellationTokenSource();
            var tcs = new TaskCompletionSource<TRes>();
            Pending[reqId] = new PendingItem
            {
                Resolve = msg => tcs.TrySetResult((TRes)msg),
                Reject = ex => tcs.TrySetException(ex),
                Cts = cts
            };
            Timer timer = null;
            timer = new Timer(_ =>
            {
                // 逾時檢查派發到主線程，避免 Response 已排入主線程佇列卻被 Timer（Thread Pool）先 Reject 的競態
                DispatchToMainThread(() =>
                {
                    timer?.Dispose();
                    if (Pending.Remove(reqId, out var p))
                    {
                        p.Reject(new TimeoutException($"[{ConnType}] Request timeout"));
                        // 不呼叫 p.Cts?.Cancel()，避免觸發 TrySetCanceled 導致 await 收到 OperationCanceledException 而非 TimeoutException
                    }
                });
            }, null, timeoutMs, Timeout.Infinite);
            cts.Token.Register(() => timer?.Dispose());
            if (route != "gate/ping")
                Debug.Log($"🔵[{ConnType}][Request][{reqId}][▶][{route}] {payload}");
            try
            {
                Send(route, reqId, bin, ex => { if (Pending.Remove(reqId, out var p)) { p.Reject(ex); p.Cts?.Cancel(); } });
            }
            catch (Exception ex)
            {
                Pending.Remove(reqId);
                tcs.TrySetException(ex);
            }
            return (tcs.Task, cts);
        }

        public void Notify<TReq>(string route, TReq payload) where TReq : IMessage
        {
            var (svt, method) = ParseRoute(route);
            var pack = new Pack
            {
                PackType = 2,
                ReqId = 0,
                Svt = svt,
                Method = method,
                Info = new Google.Protobuf.WellKnownTypes.Any
                {
                    TypeUrl = "type.googleapis.com/" + payload.Descriptor.FullName,
                    Value = ByteString.CopyFrom(payload.ToByteArray())
                }
            };
            Debug.Log($"🟠[{ConnType}][Notify][▶][{route}] {payload}");
            Send(route, 0, pack.ToByteArray(), _ => { });
        }

        public void Trigger(string route)
        {
            var (svt, method) = ParseRoute(route);
            var pack = new Pack { PackType = 3, ReqId = 0, Svt = svt, Method = method };
            Debug.Log($"🟠[{ConnType}][Trigger][▶][{route}] (no payload)");
            Send(route, 0, pack.ToByteArray(), _ => { });
        }

        public (Task<TRes> Promise, object AbortToken) Fetch<TRes>(string route, int timeoutMs = 15000) where TRes : IMessage
        {
            var (svt, method) = ParseRoute(route);
            var reqId = GetNextReqId();
            var pack = new Pack { PackType = 4, ReqId = reqId, Svt = svt, Method = method };
            var bin = pack.ToByteArray();
            var cts = new CancellationTokenSource();
            var tcs = new TaskCompletionSource<TRes>();
            Pending[reqId] = new PendingItem
            {
                Resolve = msg => tcs.TrySetResult((TRes)msg),
                Reject = ex => tcs.TrySetException(ex),
                Cts = cts
            };
            Timer timer = null;
            timer = new Timer(_ =>
            {
                DispatchToMainThread(() =>
                {
                    timer?.Dispose();
                    if (Pending.Remove(reqId, out var p))
                    {
                        p.Reject(new TimeoutException($"[{ConnType}] Request timeout"));
                    }
                });
            }, null, timeoutMs, Timeout.Infinite);
            cts.Token.Register(() => timer?.Dispose());
            Debug.Log($"🔵[{ConnType}][Fetch][{reqId}][▶][{route}] (no payload)");
            try { Send(route, reqId, bin, ex => { if (Pending.Remove(reqId, out var p)) p.Reject(ex); }); }
            catch (Exception ex) { Pending.Remove(reqId); tcs.TrySetException(ex); }
            return (tcs.Task, cts);
        }

        public void OnNotify<T>(string route, Action<T> handler) where T : IMessage
        {
            if (!PushHandlers.ContainsKey(route)) PushHandlers[route] = new List<Delegate>();
            PushHandlers[route].Add(handler);
        }

        public void OffNotify<T>(string route, Action<T> handler) where T : IMessage
        {
            if (!PushHandlers.TryGetValue(route, out var list)) return;
            list.RemoveAll(d => ReferenceEquals(d, handler));
            if (list.Count == 0) PushHandlers.Remove(route);
        }

        public void OnNotifyErr(Action<string, string> handler) => ErrorHandlers.Add(handler);
        public void OnClose(Action<ushort, string> handler) => CloseHandlers.Add(handler);
        public void OffClose(Action<ushort, string> handler) => CloseHandlers.RemoveAll(h => (Action<ushort, string>)h == handler);
        public void OnReconnect(Action handler) => ReconnectHandlers.Add(handler);
        public void OnReconnectFailed(Action handler) => ReconnectFailedHandlers.Add(handler);
        public void OnBeforeReconnect(Func<Task> handler) => BeforeReconnectHandlers.Add(handler);

        protected void EmitClose(ushort code, string reason) { foreach (var h in CloseHandlers) try { h(code, reason); } catch (Exception ex) { UnityEngine.Debug.LogException(ex); } }
        protected void EmitReconnect() { foreach (var h in ReconnectHandlers) try { h(); } catch (Exception ex) { UnityEngine.Debug.LogException(ex); } }
        protected void EmitReconnectFailed() { foreach (var h in ReconnectFailedHandlers) try { h(); } catch (Exception ex) { UnityEngine.Debug.LogException(ex); } }
        protected async Task EmitBeforeReconnect() { foreach (var h in BeforeReconnectHandlers) await h(); }

        public virtual void StopScan() { }
        public void AbortAll(string reason = null)
        {
            var msg = reason ?? "All requests aborted";
            foreach (var kv in Pending)
            {
                try { kv.Value.Reject(new Exception(msg)); kv.Value.Cts?.Cancel(); } catch { }
            }
            Pending.Clear();
        }

        public uint GetNextReqId() => ReqIdCounter++;
        public virtual bool IsConnected() => IsOpen;
        public Task WaitForConnectionAsync() => OpenPromise;
    }
}
