package rule

import (
	"encoding/json"
	"fmt"
)

// Cache 存放從 GetGameInfo 取得的 rule（playerModes）解析與正規化結果，供 entry 驗證與取參數用。
type Cache struct {
	// normalized: playerMode -> 該 mode 值物件正規化後的 JSON 字串
	normalized map[string]string
	// params: playerMode -> 該 mode 的原始值物件（供取 minP、maxP 等）
	params map[string]map[string]interface{}
}

// NewCache 從 GetGameInfo 回傳的 rule JSON 建立 Cache。ruleJSON 為空或 "{}" 時回傳空 cache（無 playerMode）。
func NewCache(ruleJSON string) (*Cache, error) {
	modes, err := ParseRuleJSON(ruleJSON)
	if err != nil {
		return nil, fmt.Errorf("parse rule: %w", err)
	}
	c := &Cache{
		normalized: make(map[string]string),
		params:     make(map[string]map[string]interface{}),
	}
	for mode, valueObj := range modes {
		norm, err := NormalizeValueJSON(valueObj)
		if err != nil {
			return nil, fmt.Errorf("normalize mode %q: %w", mode, err)
		}
		c.normalized[mode] = norm
		c.params[mode] = valueObj
	}
	return c, nil
}

// ValidateEntryRule 驗證 entry 的 rule 字串：必須為單一 playerMode 的物件 { "<playerMode>": { ... } }，
// 且該 playerMode 存在於 cache，且值物件正規化後與 cache 內一致。成功時回傳 playerMode 與該 mode 的參數。
func (c *Cache) ValidateEntryRule(entryRuleJSON string) (playerMode string, params map[string]interface{}, err error) {
	if entryRuleJSON == "" {
		return "", nil, fmt.Errorf("entry rule 為空")
	}
	var top map[string]interface{}
	if err := json.Unmarshal([]byte(entryRuleJSON), &top); err != nil {
		return "", nil, fmt.Errorf("entry rule 非合法 JSON: %w", err)
	}
	if len(top) != 1 {
		return "", nil, fmt.Errorf("entry rule 必須恰好一個 playerMode，目前為 %d 個", len(top))
	}
	for k, v := range top {
		playerMode = k
		vm, ok := v.(map[string]interface{})
		if !ok {
			return "", nil, fmt.Errorf("entry rule 的 playerMode 值必須為物件")
		}
		norm, err := NormalizeValueJSON(vm)
		if err != nil {
			return "", nil, fmt.Errorf("正規化 entry rule: %w", err)
		}
		expect, ok := c.normalized[playerMode]
		if !ok {
			return "", nil, fmt.Errorf("playerMode %q 不存在於遊戲設定", playerMode)
		}
		if norm != expect {
			return "", nil, fmt.Errorf("rule 與遊戲設定不一致")
		}
		params = c.params[playerMode]
		break
	}
	return playerMode, params, nil
}

// GetModeParams 回傳指定 playerMode 的參數物件；不存在時 ok=false。
func (c *Cache) GetModeParams(mode string) (map[string]interface{}, bool) {
	p, ok := c.params[mode]
	return p, ok
}

// NormalizedString 回傳指定 playerMode 的正規化 rule 字串（供 ruleHash 等使用）；不存在時回傳空字串。
func (c *Cache) NormalizedString(mode string) string {
	return c.normalized[mode]
}

// Modes 回傳所有已快取的 playerMode 名稱（順序未定義）。
func (c *Cache) Modes() []string {
	modes := make([]string, 0, len(c.params))
	for m := range c.params {
		modes = append(modes, m)
	}
	return modes
}
