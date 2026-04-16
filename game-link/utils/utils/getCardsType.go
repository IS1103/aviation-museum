// 牌編碼：0-51，card/4 = 點數(0=A, 1=2, ..., 12=K)，card%4 = 花色(0=♠, 1=♥, 2=♦, 3=♣)

package utils

import (
	"sort"
)

// 返回 true 表示新牌比舊牌大
func WhoIsBigger(newType, newBigOne, prvType, prvBigOne int32) bool {
	// fmt.Printf("newType: %d, newBigOne: %d, prvType: %d, prvBigOne: %d\n", newType, newBigOne, prvType, prvBigOne)
	// 單張、對子、三張必須是相同牌型，直接比較最大牌
	if newType <= 3 && prvType <= 3 {
		if newType != prvType {
			return false
		}
		return CompareCard(newBigOne, prvBigOne) > 0
	}

	// 五張牌的特殊牌型（順子、同花、葫蘆、鐵支、同花順）
	// 牌型等級：同花順(8) > 鐵支(7) > 葫蘆(6) > 同花(5) > 順子(4)
	if newType >= 4 && prvType >= 4 {
		// 牌型不同，比較牌型等級
		if newType != prvType {
			return newType > prvType
		}
		// 牌型相同，比較最大牌
		return CompareCard(newBigOne, prvBigOne) > 0
	}

	// 其他情況無法比較
	return false
}

// GetCardsType 取得牌型和最大牌
// 返回：(牌型, 最大牌)
// 牌型：0=無效, 1=單張, 2=對子, 3=三張, 4=順子, 5=同花, 6=葫蘆, 7=鐵支, 8=同花順
func GetCardsType(cards []int32) (cardsType int32, bigOne int32) {
	if len(cards) == 0 {
		return 0, 0
	}

	switch len(cards) {
	case 1:
		// 單張
		return 1, cards[0]

	case 2:
		// 對子
		if isPair(cards) {
			return 2, getMaxCardWith2High(cards)
		}
		return 0, getMaxCardWith2High(cards)

	case 3:
		// 三張
		if isTriple(cards) {
			return 3, getMaxCardWith2High(cards)
		}
		return 0, getMaxCardWith2High(cards)

	case 5:
		// 五張牌的特殊牌型
		if isStraightFlush(cards) {
			return 8, getMaxCardWith2High(cards)
		}
		if isFourOfAKind(cards) {
			return 7, getMaxCardWith2High(cards)
		}
		if isFullHouse(cards) {
			return 6, getMaxCardWith2High(cards)
		}
		if isFlush(cards) {
			return 5, getMaxCardWith2High(cards)
		}
		if isStraight(cards) {
			return 4, getMaxCardWith2High(cards)
		}
		return 0, getMaxCardWith2High(cards)

	default:
		// 其他數量無效
		return 0, 0
	}
}

// isPair 檢查是否為對子
func isPair(cards []int32) bool {
	if len(cards) != 2 {
		return false
	}
	return cards[0]/4 == cards[1]/4
}

// isTriple 檢查是否為三張
func isTriple(cards []int32) bool {
	if len(cards) != 3 {
		return false
	}
	return cards[0]/4 == cards[1]/4 && cards[1]/4 == cards[2]/4
}

// 🂡 同花順
func isStraightFlush(cards []int32) bool {
	if len(cards) < 5 {
		return false
	}
	return isFlush(cards) && isStraight(cards)
}

// 💎 同花
func isFlush(cards []int32) bool {
	if len(cards) < 5 {
		return false
	}
	suit := cards[0] % 4
	for _, c := range cards {
		if c%4 != suit {
			return false
		}
	}
	return true
}

// 🐍 順子（含 A=1 或 A=14）
func isStraight(cards []int32) bool {
	if len(cards) < 5 {
		return false
	}
	nums := []int{}
	for _, c := range cards {
		num := int(c/4) + 1
		nums = append(nums, num)
	}
	return isSequential(nums)
}

// 判斷是否為連續數字，支援 A 當 1 或 14
func isSequential(nums []int) bool {
	if len(nums) == 1 {
		return false
	}
	set := map[int]bool{}
	for _, n := range nums {
		set[n] = true
	}
	if len(set) != len(nums) {
		return false // 有重複
	}

	sorted := append([]int{}, nums...)
	sort.Ints(sorted)
	ok := true
	for i := 1; i < len(sorted); i++ {
		if sorted[i] != sorted[i-1]+1 {
			ok = false
			break
		}
	}
	if ok {
		return true
	}

	// 嘗試 A=14 的順子（如 10 J Q K A）
	converted := []int{}
	for _, n := range nums {
		if n == 1 {
			converted = append(converted, 14)
		} else {
			converted = append(converted, n)
		}
	}
	sort.Ints(converted)
	for i := 1; i < len(converted); i++ {
		if converted[i] != converted[i-1]+1 {
			return false
		}
	}
	return true
}

// 🔷 四條
func isFourOfAKind(cards []int32) bool {
	if len(cards) <= 4 {
		return false
	}
	counts := countCardNumbers(cards)
	for _, cnt := range counts {
		if cnt == 4 {
			return true
		}
	}
	return false
}

// 🎩 葫蘆
func isFullHouse(cards []int32) bool {
	if len(cards) < 5 {
		return false
	}
	counts := countCardNumbers(cards)
	has3, has2 := false, false
	for _, cnt := range counts {
		switch cnt {
		case 3:
			has3 = true
		case 2:
			has2 = true
		}
	}
	return has3 && has2
}

// 🧠 統計牌點數數量
func countCardNumbers(cards []int32) map[int]int {
	counts := map[int]int{}
	for _, c := range cards {
		num := int(c/4) + 1
		counts[num]++
	}
	return counts
}

// ⚖️ 取牌點數大小（2 最大，A 次之，3 最小）
func cardValue(card int32) int {
	num := int(card/4) + 1
	switch num {
	case 2:
		return 13
	case 1:
		return 12
	default:
		return num - 2 // 3 為 1，4 為 2...
	}
}

// 🔍 取得最大牌（先比點數，再比花色）
func getMaxCardWith2High(cards []int32) int32 {
	if len(cards) == 0 {
		return 0
	}
	max := cards[0] // 起始值應該是手牌第一張
	for _, c := range cards[1:] {
		if CompareCard(c, max) > 0 {
			max = c
		}
	}
	return max
}

// 比較兩張牌，valA > valB 回傳正數，a 花色大於 b 也回正數
func CompareCard(a, b int32) int {
	valA := cardValue(a)
	valB := cardValue(b)
	if valA != valB {
		return valA - valB
	}
	// ♠>♥>♦>♣，a%4 < b%4 則 a 大
	return int(b%4) - int(a%4) // 這樣寫，0最大
}

// 🔎 測試
// func main() {
// 	// 順子 A-2-3-4-5
// 	cards := []int32{0, 4, 8, 12, 16} // 黑桃A, 黑桃2, 黑桃3, 黑桃4, 黑桃5
// 	ctype, big := GetCardsType(cards)
// 	fmt.Printf("牌型: %d, 最大: %d\n", ctype, big)
// }

func InputTests(pairs [][2]string) []int32 {
	res := make([]int32, len(pairs))
	for i, p := range pairs {
		res[i] = InputTest(p[0], p[1])
	}
	return res
}

func InputTest(suit, num string) int32 {
	suits := map[string]int32{"♠": 0, "♥": 1, "♦": 2, "♣": 3}
	values := map[string]int32{
		"A": 0, "2": 1, "3": 2, "4": 3, "5": 4,
		"6": 5, "7": 6, "8": 7, "9": 8, "10": 9,
		"J": 10, "Q": 11, "K": 12,
	}
	s, ok1 := suits[suit]
	v, ok2 := values[num]
	if !ok1 || !ok2 {
		panic("花色或點數錯誤")
	}
	return v*4 + s
}

func CardIDToSymbol(cardID int32) string {
	suits := []string{"♠", "♥", "♦", "♣"} // 0,1,2,3
	values := []string{"A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"}

	suit := suits[cardID%4]
	value := values[cardID/4]

	return suit + value
}

func CardsToSymbols(cards []int32) []string {
	symbols := make([]string, len(cards))
	if len(cards) > 0 {
		for i, c := range cards {
			symbols[i] = CardIDToSymbol(c)
		}
	}
	return symbols
}
