# 在 Unity 中使用 AirMuseumService

## 前置說明

- **建議**使用 **GameLinkClientRunner**（MonoBehaviour）接收並處理封包：掛在與呼叫連線的 GameObject 上（例如 ProjectorClient 所在物件），並將它傳入 **ConnectAsync(wsUrl, httpBaseUrl, runner)**（`httpBaseUrl` 可傳 `null`，僅保留相容）。封包與逾時回調會由該 MonoBehaviour 的 Update 在主線程依序執行，生命週期與 GameObject 一致。
- 若不傳 runner（或傳 null），則使用全域 **GameLinkMainThreadDispatcher**（遊戲啟動時自動建立）。
- 所有 **OnPlayer / OnState / OnError** 回調都會在 **Unity 主線程** 觸發，可直接操作 GameObject、UI。
- 錯誤**不會拋出**，一律經 **OnError(string msg)** 通知。

---

## 基本流程

1. 取得單例 → 訂閱事件（建議在 `Awake` 或 `Start`）
2. 呼叫 **ConnectAsync(wsUrl)** 或 **ConnectAsync(wsUrl, httpBaseUrl)** 建立 WebSocket 連線
3. 連線成功後呼叫 **AuthAsync(ValidateReq)** 認證
4. 依身分使用：
   - **玩家端**：送 **SendPlayer**（入桌 / 離桌 / 輸入），訂閱 **OnState** 收遊戲狀態
   - **投影端**：送 **SendState** 更新狀態，訂閱 **OnPlayer** 收玩家操作

---

## 1. 取得單例與訂閱事件

```csharp
using AirMuseum;
using Gate;

void Awake()
{
    var svc = AirMuseumService.Instance;

    svc.OnPlayer += OnPlayerMessage;   // 收到玩家操作（投影端用）
    svc.OnState += OnStateMessage;     // 收到遊戲狀態（玩家端用）
    svc.OnError += OnErrorMessage;      // 任何錯誤
}

void OnPlayerMessage(PlayerInput p)
{
    // 已在主線程，可安全改 UI / GameObject
    switch (p.Action)
    {
        case Action.Entry:  // 有人入桌
            break;
        case Action.Leave: // 有人離桌
            break;
        case Action.Input:  // 遊戲輸入 axis_x, axis_y, seq
            break;
    }
}

void OnStateMessage(GameState s)
{
    // state = 遊戲階段，uids = 房內玩家 uid 列表
    var phase = s.State;
    foreach (var uid in s.Uids) { }
}

void OnErrorMessage(string msg)
{
    Debug.LogError("[AirMuseum] " + msg);
    // 可顯示提示、重試連線等
}
```

---

## 2. 連線與認證

```csharp
// 建議從設定或 QR 取得，這裡寫死範例
string wsUrl = "wss://your-server.com/ws";

async void Start()
{
    var svc = AirMuseumService.Instance;
    // 建議：用 GameLinkClientRunner 在主線程處理封包（同 GameObject 上若無則會自動加）
    var runner = GetComponent<GameLink.Libs.Client.GameLinkClientRunner>() ?? gameObject.AddComponent<GameLink.Libs.Client.GameLinkClientRunner>();

    await svc.ConnectAsync(wsUrl, null, runner);
    if (!svc.IsConnected)
        return; // 連線失敗會已透過 OnError 通知

    // 組裝認證參數（token 由你們後端／登入流程提供）
    var payload = new ValidateReq
    {
        Token = "your-jwt-or-key=uid&device=player",
        GateSid = "",
        Device = "player"  // 或 "projector"
    };
    await svc.AuthAsync(payload);
}
```

---

## 3. 玩家端：入桌、離桌、送輸入

```csharp
// 入桌
var entry = new PlayerInput { Action = Action.Entry };
AirMuseumService.Instance.SendPlayer(entry);

// 離桌
var leave = new PlayerInput { Action = Action.Leave };
AirMuseumService.Instance.SendPlayer(leave);

// 遊戲中輸入（約 15–20 Hz，axis -1～1、seq 遞增）
var input = new PlayerInput
{
    Action = Action.Input,
    AxisX = 0.5f,
    AxisY = -0.3f,
    Seq = seq++
};
AirMuseumService.Instance.SendPlayer(input);
```

---

## 4. 投影端：送遊戲狀態

```csharp
var state = new GameState
{
    State = GamePhase.Playing  // WaitingEntry / Demo / Playing / Full
};
AirMuseumService.Instance.SendState(state);
```

---

## 5. 注意事項

- **事件僅支援 +=**，不提供 unsubscribe API；若腳本會被 Destroy，可在 `OnDestroy` 裡避免在回調內操作已銷毀的物件（例如先設 flag 或改用弱參考）。
- **ConnectAsync** 僅 **wsUrl** 必填；第二參數 **httpBaseUrl** 可省略或傳 `null`（保留舊呼叫相容）。
- 投影端 device=projector 認證後即入房；玩家端 device=player 認證後需再送 **SendPlayer(Entry)** 入桌。

---

## 6. 若無法連線（Unable to connect to the remote server）

- **wsUrl 必須含路徑**：服務端 WebSocket 掛在 `/ws`，請使用 `ws://host:8770/ws`，不要用 `ws://host:8770`。
- **Unity 在 Windows、服務在 WSL**：Windows 的 `localhost` 不會指到 WSL，連線會失敗。請在 WSL 終端執行 `hostname -I` 取得 IP（例如 `172.18.0.2`），在 Unity 裡將 **wsUrl** 改為 `ws://172.18.0.2:8770/ws` 後再試。
- 確認服務已啟動且日誌顯示 `WS: 8770`；必要時檢查防火牆是否放行 8770（若仍使用 HTTP 埠則一併放行 8771）。

- 投影端腳本：**Assets/Script/ProjectorClient.cs**（可掛在 GameObject 上）。
- 手機端／玩家端腳本：**Assets/Script/PlayerClient.cs**（可掛在 GameObject 上，提供入桌／離桌／送輸入與選填 UID、狀態 Text）。
