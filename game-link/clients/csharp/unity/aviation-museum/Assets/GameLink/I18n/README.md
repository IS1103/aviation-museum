# GameLink I18n（多國語言）

與 Cocos 端共用同一套 CSV 格式與 key 命名，語料檔格式一致。

## 語料檔

- **路徑**：`Assets/Resources/lan.csv`（與 Cocos 的 `assets/resources/lan.csv` 對應）
- **格式**：第一列標題 `,zh-Hant,en-US`，之後每列 `key, 繁中, 英文`

## 使用方式

1. **初始化**（建議在進入主流程前呼叫一次）  
   - 同步：`GameLink.I18n.I18n.Instance.Init();`  
   - 非同步：`await GameLink.I18n.I18n.Instance.InitAsync();`

2. **設定語系**（依登入或設定）  
   - `GameLink.I18n.I18n.Instance.SetLocale("zh-Hant");` 或 `"en-US"`

3. **取文案**  
   - `GameLink.I18n.I18n.Instance.T("loading.btn_confirm");`

4. **UI 綁定**  
   - **I18nLabel**：掛在含 `Text` 的 GameObject 上，Inspector 設 `key`（如 `loading.btn_confirm`），執行時會自動顯示對應翻譯；切語系後呼叫 `Refresh()`。  
   - **I18nSprite**：掛在含 `Image` 的 GameObject 上，Inspector 設 `Locales` 與 `Sprites` 陣列一一對應，會依目前語系顯示對應圖；切語系後呼叫 `Refresh()`。

## 與 Cocos 對照

| Cocos | Unity |
|-------|--------|
| `await i18n.init()` | `await I18n.Instance.InitAsync()` 或 `I18n.Instance.Init()` |
| `i18n.setLocale(locale)` | `I18n.Instance.SetLocale(locale)` |
| `i18n.t('loading.btn_confirm')` | `I18n.Instance.T("loading.btn_confirm")` |
| I18nLabel 組件 + key | I18nLabel 組件 + key |
| I18nSprite 組件 + locales / spriteFrames | I18nSprite 組件 + locales / sprites |
