package engine

import (
	"errors"
	"fmt"
	"math/rand"
)

var (
	ErrGameOver      = errors.New("游戏已经结束")
	ErrNotYourTurn   = errors.New("还没轮到你")
	ErrWrongPhase    = errors.New("当前阶段不能做这个动作")
	ErrBadIndex      = errors.New("牌的位置不对")
	ErrBadPlayer     = errors.New("玩家不存在")
	ErrAlreadyPeeked = errors.New("你已经看过牌了")
	ErrBadPeekCount  = errors.New("开局要选正好 2 张牌")
	ErrNoDiscard     = errors.New("弃牌堆是空的")
	ErrBadClaimCount = errors.New("最多只能一次换出 3 张牌")
	ErrNoPower       = errors.New("这张牌没有能力")
	ErrBadTarget     = errors.New("目标不对")
	ErrNoPending     = errors.New("没有待处理的牌")
	ErrCaboLocked    = errors.New("已经喊过 CABO，不能再喊")
	ErrLockedHand    = errors.New("喊 CABO 的人手牌已锁定")
	ErrDeckEmpty     = errors.New("牌堆空了")
)

// Phase 对局阶段。
type Phase string

const (
	PhasePeek     Phase = "peek"     // 开局看牌
	PhaseTurn     Phase = "turn"     // 等待行动
	PhasePlace    Phase = "place"    // 已摸牌，等待安置
	PhaseRoundEnd Phase = "roundEnd" // 本轮结算
	PhaseGameOver Phase = "gameover"
)

// Slot 手牌位。FaceUp 表示这张牌对所有人公开（从弃牌堆拿的牌）。
// KnownBy 记录哪些玩家知道这张牌的内容。
type Slot struct {
	Card    Card  `json:"card"`
	FaceUp  bool  `json:"faceUp"`
	KnownBy []int `json:"knownBy"`
}

func (s Slot) KnownByPlayer(p int) bool {
	for _, k := range s.KnownBy {
		if k == p {
			return true
		}
	}
	return false
}

// Player 玩家。
type Player struct {
	Name      string `json:"name"`
	IsAI      bool   `json:"isAI"`
	Hand      []Slot `json:"hand"`
	Score     int    `json:"score"`
	RoundPts  int    `json:"roundPts"`
	ResetUsed bool   `json:"resetUsed"`
}

// Target 指向某个玩家的某个牌位。
type Target struct {
	Player int `json:"player"`
	Slot   int `json:"slot"`
}

// State 完整对局状态。引擎内部持有全部信息，对外必须走 ViewFor 裁剪。
type State struct {
	Players   []Player `json:"players"`
	Deck      []Card   `json:"deck"`
	Discard   []Card   `json:"discard"`
	Turn      int      `json:"turn"`
	Phase     Phase    `json:"phase"`
	Caller    int      `json:"caller"` // -1 表示还没人喊 CABO
	Pending   *Card    `json:"pending"`
	CaboQueue []int    `json:"caboQueue"`
	Peeked    []bool   `json:"peeked"`
	Round     int      `json:"round"`
	Winner    int      `json:"winner"` // -1 表示未结束
	Log       []string `json:"log"`
	Events    []Event  `json:"events"`
}

const maxLog = 40

func (s *State) addLog(format string, args ...any) {
	s.Log = append(s.Log, fmt.Sprintf(format, args...))
	if len(s.Log) > maxLog {
		s.Log = s.Log[len(s.Log)-maxLog:]
	}
}

// Clone 深拷贝，保证引擎调用方拿到的旧状态不被修改。
func (s State) Clone() State {
	c := s
	c.Players = make([]Player, len(s.Players))
	for i, p := range s.Players {
		c.Players[i] = p
		c.Players[i].Hand = make([]Slot, len(p.Hand))
		for j, sl := range p.Hand {
			c.Players[i].Hand[j] = sl
			c.Players[i].Hand[j].KnownBy = append([]int(nil), sl.KnownBy...)
		}
	}
	c.Deck = append([]Card(nil), s.Deck...)
	c.Discard = append([]Card(nil), s.Discard...)
	c.Peeked = append([]bool(nil), s.Peeked...)
	c.CaboQueue = append([]int(nil), s.CaboQueue...)
	c.Log = append([]string(nil), s.Log...)
	c.Events = append([]Event(nil), s.Events...)
	if s.Pending != nil {
		p := *s.Pending
		c.Pending = &p
	}
	return c
}

// NewGame 开一局新游戏。names 与 ai 长度需一致。
func NewGame(names []string, ai []bool, r *rand.Rand) State {
	n := len(names)
	s := State{
		Players: make([]Player, n),
		Caller:  -1,
		Winner:  -1,
		Round:   1,
	}
	for i := 0; i < n; i++ {
		s.Players[i] = Player{Name: names[i], IsAI: ai[i]}
	}
	s.startRound(r)
	return s
}

// startRound 发牌并重置本轮状态。
func (s *State) startRound(r *rand.Rand) {
	deck := NewDeck()
	r.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })

	s.Deck = deck
	s.Discard = nil
	s.Pending = nil
	s.CaboQueue = nil
	s.Caller = -1
	s.Turn = 0
	s.Phase = PhasePeek
	s.Peeked = make([]bool, len(s.Players))

	for i := range s.Players {
		s.Players[i].Hand = nil
		s.Players[i].RoundPts = 0
		for k := 0; k < 4; k++ {
			n := len(s.Deck)
			s.Players[i].Hand = append(s.Players[i].Hand, Slot{Card: s.Deck[n-1]})
			s.Deck = s.Deck[:n-1]
		}
	}
	s.addLog("—— 第 %d 轮开始，每人 4 张牌 ——", s.Round)
	s.emit(Event{Kind: EvRoundStart, Player: -1, Value: s.Round})
}

func (s *State) allPeeked() bool {
	for _, p := range s.Peeked {
		if !p {
			return false
		}
	}
	return true
}

// advance 推进到下一个行动者。若有 CABO 队列则按队列走。
func (s *State) advance() {
	s.Phase = PhaseTurn
	if s.Caller >= 0 {
		if len(s.CaboQueue) > 0 {
			s.CaboQueue = s.CaboQueue[1:]
		}
		if len(s.CaboQueue) == 0 {
			s.endRound()
			return
		}
		s.Turn = s.CaboQueue[0]
		return
	}
	s.Turn = (s.Turn + 1) % len(s.Players)
}

// endRound 结算本轮。
func (s *State) endRound() {
	n := len(s.Players)
	totals := make([]int, n)
	for i := range s.Players {
		sum := 0
		for _, sl := range s.Players[i].Hand {
			sum += sl.Card.Points()
		}
		totals[i] = sum
	}

	min := totals[0]
	for _, t := range totals {
		if t < min {
			min = t
		}
	}

	gains := make([]int, n)
	for i := range s.Players {
		switch {
		case i == s.Caller && totals[i] == min:
			gains[i] = 0
		case i == s.Caller:
			gains[i] = totals[i] + 10
		default:
			gains[i] = totals[i]
		}
	}

	s.addLog("本轮结算：")
	for i := range s.Players {
		ns := s.Players[i].Score + gains[i]
		if ns == 100 && !s.Players[i].ResetUsed {
			ns = 50
			s.Players[i].ResetUsed = true
			s.addLog("  %s 分数正好 100，重置为 50（本场限一次）", s.Players[i].Name)
			// 这条必须单独发事件：分数从 100 跳回 50 若没有任何解释，玩家会以为是 bug
			s.emit(Event{Kind: EvReset, Player: i, Value: ns})
		}
		s.Players[i].Score = ns
		s.Players[i].RoundPts = gains[i]
		s.addLog("  %s：手牌 %d 点，本轮 +%d，总分 %d", s.Players[i].Name, totals[i], gains[i], ns)
	}

	s.Pending = nil
	s.CaboQueue = nil

	for i := range s.Players {
		s.emit(Event{Kind: EvScore, Player: i, Value: gains[i]})
	}

	over := false
	for i := range s.Players {
		if s.Players[i].Score > 100 {
			over = true
		}
	}
	if over {
		best := 0
		for i := range s.Players {
			if s.Players[i].Score < s.Players[best].Score {
				best = i
			}
		}
		s.Winner = best
		s.Phase = PhaseGameOver
		s.emit(Event{Kind: EvGameOver, Player: best})
		s.addLog("游戏结束！%s 以 %d 分获胜", s.Players[best].Name, s.Players[best].Score)
		return
	}
	s.Phase = PhaseRoundEnd
}

// NextRound 进入下一轮。
func NextRound(s State, r *rand.Rand) State {
	c := s.Clone()
	if c.Phase != PhaseRoundEnd {
		return c
	}
	c.Round++
	c.startRound(r)
	return c
}
