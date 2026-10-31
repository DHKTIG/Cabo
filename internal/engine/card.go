package engine

import "strconv"

// Power 技能类型。
type Power int

const (
	PowerNone Power = iota
	PowerPeek       // 看自己一张
	PowerSpy        // 看对手一张
	PowerSwap       // 交换场上任意两张
)

func (p Power) String() string {
	switch p {
	case PowerPeek:
		return "peek"
	case PowerSpy:
		return "spy"
	case PowerSwap:
		return "swap"
	}
	return ""
}

// Label 卡面上显示的单字。
func (p Power) Label() string {
	switch p {
	case PowerPeek:
		return "看"
	case PowerSpy:
		return "窥"
	case PowerSwap:
		return "换"
	}
	return ""
}

// Name 完整技能名，用于提示文案。
func (p Power) Name() string {
	switch p {
	case PowerPeek:
		return "透视"
	case PowerSpy:
		return "窥探"
	case PowerSwap:
		return "置换"
	}
	return ""
}

// Card 一张牌。牌面只有数字，没有花色。
// ID 全局唯一，用于追踪单张牌的流转与牌数守恒断言。
type Card struct {
	ID   int `json:"id"`
	Rank int `json:"rank"`
}

// Points 数字即点数。
func (c Card) Points() int { return c.Rank }

// Label 牌面数字。
func (c Card) Label() string { return strconv.Itoa(c.Rank) }

// Power 7/8 透视，9/10 窥探，J/Q（11/12）置换，其余无技能。
func (c Card) Power() Power {
	switch c.Rank {
	case 7, 8:
		return PowerPeek
	case 9, 10:
		return PowerSpy
	case 11, 12:
		return PowerSwap
	}
	return PowerNone
}

// NewDeck 一副 54 张牌，与标准扑克牌点数分布完全一致，但去掉花色：
//
//	1–12 各 4 张（48）
//	13   共 3 张
//	0    共 3 张（原本的方块 K 与两张王）
//
// 总点数 4×(1+…+12) + 3×13 = 351，平均 6.5 分，和原版一模一样。
func NewDeck() []Card {
	cards := make([]Card, 0, 54)
	id := 0
	add := func(rank, n int) {
		for i := 0; i < n; i++ {
			cards = append(cards, Card{ID: id, Rank: rank})
			id++
		}
	}
	for r := 1; r <= 12; r++ {
		add(r, 4)
	}
	add(13, 3)
	add(0, 3)
	return cards
}
