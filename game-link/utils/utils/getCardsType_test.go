package utils

import (
	"fmt"
	"testing"
)

type cardsCase struct {
	name     string
	cards    [][2]string // 花色＋點數組
	wantType int32
	wantMax  [2]string // 最大牌的花色與點數（可選，僅列印用）
}

func TestGetCardsType(t *testing.T) {
	cases := []cardsCase{
		{
			name: "同花順", // ♠A, ♠2, ♠3, ♠4, ♠5
			cards: [][2]string{
				{"♠", "A"}, {"♠", "2"}, {"♠", "3"}, {"♠", "4"}, {"♠", "5"},
			},
			wantType: 8,
			wantMax:  [2]string{"♠", "A"},
		},
		{
			name: "四條", // ♠2, ♥2, ♦2, ♣2, ♠A
			cards: [][2]string{
				{"♠", "2"}, {"♥", "2"}, {"♦", "2"}, {"♣", "2"}, {"♠", "A"},
			},
			wantType: 7,
			wantMax:  [2]string{"♠", "2"},
		},
		{
			name: "葫蘆", // ♠3, ♥3, ♦3, ♠4, ♥4
			cards: [][2]string{
				{"♠", "3"}, {"♥", "3"}, {"♦", "3"}, {"♠", "4"}, {"♥", "4"},
			},
			wantType: 6,
			wantMax:  [2]string{"♠", "4"},
		},
		{
			name: "同花", // ♣8, ♣2, ♣5, ♣K, ♣J
			cards: [][2]string{
				{"♣", "8"}, {"♣", "2"}, {"♣", "5"}, {"♣", "K"}, {"♣", "J"},
			},
			wantType: 5,
			wantMax:  [2]string{"♣", "K"},
		},
		{
			name: "順子", // ♥9, ♣10, ♠J, ♦Q, ♥K
			cards: [][2]string{
				{"♥", "9"}, {"♣", "10"}, {"♠", "J"}, {"♦", "Q"}, {"♥", "K"},
			},
			wantType: 4,
			wantMax:  [2]string{"♥", "K"},
		},
		{
			name: "A2345順", // ♠A, ♥2, ♦3, ♣4, ♠5
			cards: [][2]string{
				{"♠", "A"}, {"♥", "2"}, {"♦", "3"}, {"♣", "4"}, {"♠", "5"},
			},
			wantType: 4,
			wantMax:  [2]string{"♠", "A"},
		},
		{
			name: "雜牌", // ♠A, ♥7, ♦5, ♣10, ♠K
			cards: [][2]string{
				{"♠", "A"}, {"♥", "7"}, {"♦", "5"}, {"♣", "10"}, {"♠", "K"},
			},
			wantType: 0,
			wantMax:  [2]string{"♠", "A"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cardInts := InputTests(c.cards)
			gotType, gotMax := GetCardsType(cardInts)
			wantMaxID := InputTest(c.wantMax[0], c.wantMax[1])
			gotMaxSym := CardIDToSymbol(gotMax)
			wantMaxSym := CardIDToSymbol(wantMaxID)
			// 驗證牌型
			if gotType != c.wantType {
				t.Errorf("[%s] 預期牌型=%d，實際=%d (%v)", c.name, c.wantType, gotType, CardsToSymbols(cardInts))
			}
			// 印出
			fmt.Printf("【%s】%v → 牌型: %d 最大: %s (預期最大: %s)\n",
				c.name, CardsToSymbols(cardInts), gotType, gotMaxSym, wantMaxSym)
		})
	}
}

func TestOneCase(t *testing.T) {
	cases := []cardsCase{
		{
			name: "同花順", // ♠A, ♠2, ♠3, ♠4, ♠5
			cards: [][2]string{
				{"♠", "A"}, {"♠", "2"}, {"♠", "3"}, {"♠", "4"}, {"♠", "5"},
			},
			wantType: 8,
			wantMax:  [2]string{"♠", "A"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cardInts := InputTests(c.cards)
			gotType, gotMax := GetCardsType(cardInts)
			wantMaxID := InputTest(c.wantMax[0], c.wantMax[1])
			gotMaxSym := CardIDToSymbol(gotMax)
			wantMaxSym := CardIDToSymbol(wantMaxID)
			// 驗證牌型
			if gotType != c.wantType {
				t.Errorf("[%s] 預期牌型=%d，實際=%d (%v)", c.name, c.wantType, gotType, CardsToSymbols(cardInts))
			}
			// 印出
			fmt.Printf("【%s】%v → 牌型: %d 最大: %s (預期最大: %s)\n",
				c.name, CardsToSymbols(cardInts), gotType, gotMaxSym, wantMaxSym)
		})
	}
}

func TestInputTests(t *testing.T) {
	// 測試資料
	pairs := [][2]string{
		{"♠", "A"},
		{"♠", "2"},
		{"♠", "3"},
		{"♥", "K"},
		{"♦", "10"},
		{"♣", "J"},
	}

	// 產生牌 int32
	cards := InputTests(pairs)
	fmt.Printf("Input: %v\nOutput int32: %v\n", pairs, cards)

	// 轉回花色點數符號
	symbols := CardsToSymbols(cards)
	fmt.Printf("轉回符號: %v\n", symbols)

	// 驗證輸出與輸入
	for i, p := range pairs {
		expect := p[0] + p[1]
		if symbols[i] != expect {
			t.Errorf("第%d張, 預期 %s, 實際 %s", i, expect, symbols[i])
		}
	}
}

func TestIsBigger(t *testing.T) {
	a := InputTests([][2]string{
		{"♣", "A"}, //{"♠", "A"}, {"♠", "2"}, {"♠", "3"}, {"♠", "4"}, {"♠", "5"},
	})
	b := InputTests([][2]string{
		{"♣", "3"}, //{"♠", "9"}, {"♠", "10"}, {"♠", "J"}, {"♠", "Q"}, {"♠", "K"},
	})
	fmt.Printf("a: %v\n", CardsToSymbols(a))
	fmt.Printf("a: %v\n", a)
	// fmt.Printf("b: %v\n", CardsToSymbols(b))
	// fmt.Printf("b: %v\n", b)
	aType, aMax := GetCardsType(a)
	bType, bMax := GetCardsType(b)
	// fmt.Printf("bType: %d, bMax: %d\n", bType, bMax)
	res := WhoIsBigger(aType, aMax, bType, bMax)
	fmt.Printf("IsBigger(%v, %v) = %v\n", CardsToSymbols(a), CardsToSymbols(b), res)
	if !res {
		t.Errorf("a 應該比 b 大")
	}
}

func TestCompareCard(t *testing.T) {
	type compareTest struct {
		a      [2]string // a: 花色+點數
		b      [2]string // b: 花色+點數
		expect int       // >0:a大, <0:b大, =0:相同
	}
	tests := []compareTest{
		// 點數比較
		{[2]string{"♣", "3"}, [2]string{"♣", "A"}, -1}, // 2 > A
		// {[2]string{"♠", "A"}, [2]string{"♠", "2"}, -1}, // A < 2
		// {[2]string{"♦", "5"}, [2]string{"♣", "3"}, 1},  // 5 > 3

		// 花色比較（同點數）
		// {[2]string{"♠", "A"}, [2]string{"♥", "A"}, 1},  // ♠ > ♥
		// {[2]string{"♥", "A"}, [2]string{"♦", "A"}, 1},  // ♥ > ♦
		// {[2]string{"♦", "A"}, [2]string{"♣", "A"}, 1},  // ♦ > ♣
		// {[2]string{"♣", "A"}, [2]string{"♠", "A"}, -1}, // ♣ < ♠
		// {[2]string{"♠", "3"}, [2]string{"♠", "3"}, 0},  // 相同
	}

	for _, tt := range tests {
		aID := InputTest(tt.a[0], tt.a[1])
		bID := InputTest(tt.b[0], tt.b[1])
		result := CompareCard(aID, bID)
		fmt.Printf("%s vs %s => result: %d\n", CardIDToSymbol(aID), CardIDToSymbol(bID), result)
		if (result > 0 && tt.expect <= 0) ||
			(result < 0 && tt.expect >= 0) ||
			(result == 0 && tt.expect != 0) {
			t.Errorf("%s vs %s: 預期 %d, 實際 %d", CardIDToSymbol(aID), CardIDToSymbol(bID), tt.expect, result)
		}
	}
}

func TestCardIDToSymbol(t *testing.T) {
	for id := int32(0); id < 52; id++ {
		s := CardIDToSymbol(id)
		if s == "" {
			t.Errorf("CardIDToSymbol(%d) 應有值", id)
		}
	}
}
