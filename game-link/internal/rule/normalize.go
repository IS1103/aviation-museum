// Package rule 提供 playerModes rule 的解析、正規化（key 依 Unicode 碼點升序）與 entry 驗證。
// 與 plan-getgameinfo-and-rule-structure 一致：Client 與 Game 使用同一套正規化規則。
package rule

import (
	"encoding/json"
	"sort"
)

// lessRuneSlice 比較兩個 rune slice 為 Unicode 碼點升序（用於 key 排序）。
func lessRuneSlice(a, b []rune) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

// sortedKeysByCodePoint 回傳 m 的所有 key，依 Unicode 碼點升序排列。
func sortedKeysByCodePoint(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return lessRuneSlice([]rune(keys[i]), []rune(keys[j]))
	})
	return keys
}

// normalizeValue 將單一 playerMode 的值物件（map[string]interface{}）正規化：
// key 依 Unicode 碼點升序排列後序列化為 JSON。空 map 回傳 "{}"。
func normalizeValue(m map[string]interface{}) (string, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	keys := sortedKeysByCodePoint(m)
	b := buildSortedJSON(keys, m)
	return string(b), nil
}

// buildSortedJSON 將 map 依 key 的碼點順序序列化為 JSON（僅一層；值用標準 json.Marshal）。
func buildSortedJSON(keys []string, m map[string]interface{}) []byte {
	b := []byte{'{'}
	for i, k := range keys {
		if i > 0 {
			b = append(b, ',')
		}
		kb, _ := json.Marshal(k)
		b = append(b, kb...)
		b = append(b, ':')
		v := m[k]
		if v == nil {
			b = append(b, "null"...)
			continue
		}
		vb, _ := json.Marshal(v)
		b = append(b, vb...)
	}
	b = append(b, '}')
	return b
}

// NormalizeValueJSON 將已解析的單一 playerMode 值物件正規化為 JSON 字串（key 依 Unicode 碼點升序）。
// 若 v 為 nil 或非 map，回傳 "{}" 與 nil error。
func NormalizeValueJSON(m map[string]interface{}) (string, error) {
	if m == nil {
		return "{}", nil
	}
	return normalizeValue(m)
}


// ParseRuleJSON 解析 GetGameInfo 回傳的 rule JSON（即 playerModes 物件）為 map[playerMode]valueObj。
func ParseRuleJSON(ruleJSON string) (map[string]map[string]interface{}, error) {
	if ruleJSON == "" {
		return nil, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(ruleJSON), &out); err != nil {
		return nil, err
	}
	result := make(map[string]map[string]interface{})
	for k, v := range out {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		result[k] = vm
	}
	return result, nil
}

