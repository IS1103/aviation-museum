package rule

import (
	"fmt"
)

// 通用欄位名稱（與 web-dashboard / game 表 player_modes 鍵名一致，camelCase）。
// 僅 minP、maxP 為跨遊戲通用欄位（湊桌與房間容量）；其餘為遊戲可選欄位
//（如 minCoin/maxCoin 為下注區間、waitingToStartTotalSec 為等開局超時等）。
const (
	FieldMinP                   = "minP"
	FieldMaxP                   = "maxP"
	FieldIsMatch                = "isMatch"
	FieldWaitingToStartTotalSec = "waitingToStartTotalSec"
	FieldMinCoin                = "minCoin"
	FieldMaxCoin                = "maxCoin"
	FieldCoinType               = "coinType" // 依此幣別檢查 entry 資產與遊玩時扣款
)

// IntParam 從 params 取整數欄位；若不存在或非數字則回傳 0 與 false。
func IntParam(params map[string]interface{}, key string) (int, bool) {
	if params == nil {
		return 0, false
	}
	v, ok := params[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}

// IntParamDefault 從 params 取整數欄位；不存在或無效時回傳 defaultVal。
func IntParamDefault(params map[string]interface{}, key string, defaultVal int) int {
	n, ok := IntParam(params, key)
	if !ok {
		return defaultVal
	}
	return n
}

// BoolParam 從 params 取布林欄位；若不存在則回傳 false 與 false。
func BoolParam(params map[string]interface{}, key string) (bool, bool) {
	if params == nil {
		return false, false
	}
	v, ok := params[key]
	if !ok || v == nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// MinPMaxP 從 params 取 minP、maxP，無則用預設（minP 至少 2，maxP 至少等於 minP）。
func MinPMaxP(params map[string]interface{}, defaultMin, defaultMax int) (minP, maxP int) {
	minP = IntParamDefault(params, FieldMinP, defaultMin)
	maxP = IntParamDefault(params, FieldMaxP, defaultMax)
	if minP <= 0 {
		minP = 2
	}
	if maxP < minP {
		maxP = minP
		if defaultMax > maxP {
			maxP = defaultMax
		}
	}
	return minP, maxP
}

// ValidateCommonFields 僅對「通用欄位」minP、maxP 做檢查：當兩者皆存在時須 minP <= maxP。
// 不檢查 minCoin、maxCoin 等遊戲可選欄位（由使用該欄位的遊戲自行驗證）。不強制必填。
func ValidateCommonFields(params map[string]interface{}) error {
	if params == nil {
		return nil
	}
	minP, hasMinP := IntParam(params, FieldMinP)
	maxP, hasMaxP := IntParam(params, FieldMaxP)
	if hasMinP && hasMaxP && minP > maxP {
		return fmt.Errorf("minP 不可大於 maxP")
	}
	_ = minP
	_ = maxP
	return nil
}
