# GameLink WebSocket Client（Unity 版）

對應 Cocos 專案 `game-link-cocos-sdk/libs/client`，提供 Gate Pack 協定的 WebSocket 客戶端。

## 結構

| 檔案 | 說明 |
|------|------|
| **IClient.cs** | 客戶端介面：Request / Notify / Trigger / Fetch、OnNotify、OnClose 等 |
| **ClientBase.cs** | 抽象基類：Pack 組裝與解析、response/push 分派、超時與 abort |
| **WebsocketClient.cs** | WebSocket 實作：連線、自動重連、收發二進位 |
| **ProtoSchemaManager.cs** | 依 typeName 解析 Pack.Info (Any) 為 IMessage（Gate、AirMuseum 等已註冊） |
| **IWebSocketTransport.cs** | 傳輸層介面，可替換實作 |
| **DefaultWebSocketTransport.cs** | 使用 `System.Net.WebSockets.ClientWebSocket` 的預設實作 |

## 依賴

- **Google.Protobuf**（Unity 套件或 NuGet）
- **Gate、AirMuseum** 等 pb：來自 `Assets/GameLink/Proto`（或 `Pb` / `Game/Pb`）

## 使用範例

```csharp
// 使用預設 Transport（System.Net.WebSockets）
var transport = new DefaultWebSocketTransport();
var client = new WebsocketClient("ws://localhost:8080/ws", transport, autoReconnect: true);

client.OnNotifyErr((route, msg) => Debug.LogError($"[{route}] {msg}"));
client.OnNotify<AirMuseum.PlayerInput>("air_museum/player", data => { /* 處理 entry/leave/input */ });
client.OnNotify<AirMuseum.GameState>("air_museum/state", data => { /* 處理 state + uids */ });

await client.ConnectAsync();
await client.WaitForConnectionAsync();

// auth
var (task, _) = client.Request<Gate.ValidateReq, Gate.ValidateResp>("auth/validate", new ValidateReq { Token = "key=1&device=player" });
var resp = await task;

// 玩家送 player（entry）
client.Notify("air_museum/player", new AirMuseum.PlayerInput { Action = AirMuseum.Action.Entry });
```

## 其他平台

- **WebGL**：`System.Net.WebSockets` 可能不可用，請實作 `IWebSocketTransport`（例如用 NativeWebSocket）並傳入 `WebsocketClient`。
