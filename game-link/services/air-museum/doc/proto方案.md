# air-museum 專屬 Proto 方案

## 定位

- **auth**：沿用既有 `gate.ValidateReq` / `gate.ValidateResp`（不在此 proto）。
- **air_museum 專屬**：僅定義本服務用到的 payload，放在 **service 內** `proto/air_museum.proto`，與 holdem、baccarat 的 service 內 proto 一致。

## 訊息一覽

| 類型 | 用途 | 說明 |
|------|------|------|
| **GamePhase**（enum） | 遊戲階段 | 可入桌、Demo、遊戲中、滿員 |
| **GameState** | pushGameState / gameStateEvent 的 info | phase 必填；countdown_sec、player_count 可選 |
| **PlayerInput** | notify/input 的 info | uid（Server 轉發時填）、axis_x、axis_y、seq |

## 與 API 對應

- `air_museum/pushGameState`（notify）：Pack.info = **GameState**
- `air_museum/gameStateEvent`（notify）：Pack.info = **GameState**
- `air_museum/entry` / `air_museum/leave`：trigger，無 payload
- `air_museum/input`（notify）：Pack.info = **PlayerInput**

## 產出

- **Go**：產出到 `services/air-museum/proto/pb/`，go_package = `air-museum/proto/pb;air_museum`，服務內 import `air-museum/proto/pb`。
- **C# / TS**：若 client 需要，可由同一支 proto 用既有 tools/gen-proto 或各自產出（可之後擴充）。

## 與根目錄 proto 的關係

- 根目錄 `proto/` 僅放共用 gate、game 等；**air_museum 專屬定義僅在 service 內**，不放入根目錄 proto，以利服務邊界清晰。

## 重新產生 Go

在 `services/air-museum` 目錄下執行：

```bash
make proto
```
