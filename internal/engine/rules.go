package engine

// ActionType 动作类型。
type ActionType string

const (
	ActPeek           ActionType = "peek"           // 开局看 2 张
	ActDrawDeck       ActionType = "drawDeck"       // 从牌堆摸一张
	ActTakeDiscard    ActionType = "takeDiscard"    // 从弃牌堆拿（1 换 1 / 1 换多）
	ActDiscardPending ActionType = "discardPending" // 把摸到的牌直接弃掉
	ActPlace          ActionType = "place"          // 把摸到的牌换入手牌
	ActUsePower       ActionType = "usePower"       // 发动能力
	ActCallCabo       ActionType = "callCabo"       // 喊 CABO
)

// Action 一个玩家动作。
type Action struct {
	Type    ActionType `json:"type"`
	Indices []int      `json:"indices,omitempty"`
	Targets []Target   `json:"targets,omitempty"`
}

func addKnown(list []int, p int) []int {
	for _, k := range list {
		if k == p {
			return list
		}
	}
	return append(list, p)
}

func allPlayers(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// knownAfter 一张牌被移动后，谁还知道它的内容。
func knownAfter(faceUp bool, actor, n int) []int {
	if faceUp {
		return allPlayers(n)
	}
	return []int{actor}
}

// Apply 唯一的规则入口。返回新状态，不修改入参。
func Apply(s State, p int, a Action) (State, error) {
	s = s.Clone()
	if p < 0 || p >= len(s.Players) {
		return s, ErrBadPlayer
	}
	if s.Phase == PhaseGameOver {
		return s, ErrGameOver
	}

	if s.Phase == PhasePeek {
		return applyPeek(s, p, a)
	}
	if s.Turn != p {
		return s, ErrNotYourTurn
	}

	switch a.Type {
	case ActDrawDeck:
		return applyDrawDeck(s, p)
	case ActTakeDiscard:
		return applyTakeDiscard(s, p, a)
	case ActCallCabo:
		return applyCallCabo(s, p)
	case ActPlace:
		return applyPlace(s, p, a)
	case ActDiscardPending:
		return applyDiscardPending(s, p)
	case ActUsePower:
		return applyUsePower(s, p, a)
	}
	return s, ErrWrongPhase
}

func applyPeek(s State, p int, a Action) (State, error) {
	if a.Type != ActPeek {
		return s, ErrWrongPhase
	}
	if s.Peeked[p] {
		return s, ErrAlreadyPeeked
	}
	if len(a.Indices) != 2 {
		return s, ErrBadPeekCount
	}
	if a.Indices[0] == a.Indices[1] {
		return s, ErrBadIndex
	}
	for _, idx := range a.Indices {
		if idx < 0 || idx >= len(s.Players[p].Hand) {
			return s, ErrBadIndex
		}
	}
	for _, idx := range a.Indices {
		s.Players[p].Hand[idx].KnownBy = addKnown(s.Players[p].Hand[idx].KnownBy, p)
	}
	s.Peeked[p] = true
	s.emit(Event{Kind: EvPeek, Player: p})
	if s.allPeeked() {
		s.Phase = PhaseTurn
		s.Turn = 0
		s.addLog("所有人看完牌，游戏开始")
	}
	return s, nil
}

func applyDrawDeck(s State, p int) (State, error) {
	if s.Phase != PhaseTurn {
		return s, ErrWrongPhase
	}
	if len(s.Deck) == 0 {
		s.addLog("牌堆摸完了，本轮结束")
		s.endRound()
		return s, nil
	}
	c := s.Deck[len(s.Deck)-1]
	s.Deck = s.Deck[:len(s.Deck)-1]
	s.Pending = &c
	s.Phase = PhasePlace
	s.emit(Event{Kind: EvDraw, Player: p})
	return s, nil
}

func applyPlace(s State, p int, a Action) (State, error) {
	if s.Phase != PhasePlace || s.Pending == nil {
		return s, ErrNoPending
	}
	if len(a.Indices) != 1 {
		return s, ErrBadIndex
	}
	idx := a.Indices[0]
	if idx < 0 || idx >= len(s.Players[p].Hand) {
		return s, ErrBadIndex
	}
	old := s.Players[p].Hand[idx].Card
	s.Players[p].Hand[idx] = Slot{Card: *s.Pending, KnownBy: []int{p}}
	s.Discard = append(s.Discard, old)
	s.emit(Event{Kind: EvPlace, Player: p, Card: cardViewPtr(old)})
	s.addLog("%s 换入新牌，弃掉 %s", s.Players[p].Name, old.Label())
	s.Pending = nil
	s.advance()
	return s, nil
}

func applyDiscardPending(s State, p int) (State, error) {
	if s.Phase != PhasePlace || s.Pending == nil {
		return s, ErrNoPending
	}
	c := *s.Pending
	s.Discard = append(s.Discard, c)
	s.Pending = nil
	s.emit(Event{Kind: EvDiscard, Player: p, Card: cardViewPtr(c)})
	s.addLog("%s 弃掉 %s", s.Players[p].Name, c.Label())
	s.advance()
	return s, nil
}

func applyTakeDiscard(s State, p int, a Action) (State, error) {
	if s.Phase != PhaseTurn {
		return s, ErrWrongPhase
	}
	if len(s.Discard) == 0 {
		return s, ErrNoDiscard
	}
	if len(a.Indices) == 0 || len(a.Indices) > 3 {
		return s, ErrBadClaimCount
	}
	hand := s.Players[p].Hand
	seen := map[int]bool{}
	for _, idx := range a.Indices {
		if idx < 0 || idx >= len(hand) {
			return s, ErrBadIndex
		}
		if seen[idx] {
			return s, ErrBadIndex
		}
		seen[idx] = true
	}

	n := len(s.Players)
	top := s.Discard[len(s.Discard)-1]
	rank := hand[a.Indices[0]].Card.Rank
	same := true
	for _, idx := range a.Indices {
		if hand[idx].Card.Rank != rank {
			same = false
		}
	}

	if same {
		first := a.Indices[0]
		for _, idx := range a.Indices {
			if idx < first {
				first = idx
			}
		}
		firstCard := hand[first].Card
		s.Discard = s.Discard[:len(s.Discard)-1]
		removed := make([]Card, 0, len(a.Indices))
		newHand := make([]Slot, 0, len(hand)-len(a.Indices)+1)
		for i, sl := range hand {
			if i == first {
				removed = append(removed, sl.Card)
				newHand = append(newHand, Slot{Card: top, FaceUp: true, KnownBy: allPlayers(n)})
				continue
			}
			if seen[i] {
				removed = append(removed, sl.Card)
				continue
			}
			newHand = append(newHand, sl)
		}
		s.Players[p].Hand = newHand
		s.Discard = append(s.Discard, removed...)
		s.emit(Event{Kind: EvTake, Player: p, Card: cardViewPtr(top), Value: len(a.Indices)})
		s.addLog("%s 用 %d 张 %s 换走了弃牌堆的 %s", s.Players[p].Name, len(a.Indices), firstCard.Label(), top.Label())
	} else {
		s.Discard = s.Discard[:len(s.Discard)-1]
		s.Players[p].Hand = append(s.Players[p].Hand, Slot{Card: top, FaceUp: true, KnownBy: allPlayers(n)})
		for _, idx := range a.Indices {
			s.Players[p].Hand[idx].FaceUp = true
			s.Players[p].Hand[idx].KnownBy = allPlayers(n)
		}
		s.emit(Event{Kind: EvPenalty, Player: p, Card: cardViewPtr(top), Value: len(a.Indices)})
		s.addLog("%s 记错了牌，收下 %s 并亮出 %d 张", s.Players[p].Name, top.Label(), len(a.Indices))
		if len(a.Indices) >= 3 && len(s.Deck) > 0 {
			c := s.Deck[len(s.Deck)-1]
			s.Deck = s.Deck[:len(s.Deck)-1]
			s.Players[p].Hand = append(s.Players[p].Hand, Slot{Card: c, KnownBy: []int{p}})
			s.addLog("%s 一次换 3 张失败，额外罚抽 1 张", s.Players[p].Name)
		}
	}

	s.advance()
	return s, nil
}

func applyUsePower(s State, p int, a Action) (State, error) {
	if s.Phase != PhasePlace || s.Pending == nil {
		return s, ErrNoPending
	}
	pw := s.Pending.Power()
	if pw == PowerNone {
		return s, ErrNoPower
	}
	n := len(s.Players)

	switch pw {
	case PowerPeek:
		if len(a.Targets) != 1 {
			return s, ErrBadTarget
		}
		t := a.Targets[0]
		if t.Player != p || t.Slot < 0 || t.Slot >= len(s.Players[p].Hand) {
			return s, ErrBadTarget
		}
		s.Players[p].Hand[t.Slot].KnownBy = addKnown(s.Players[p].Hand[t.Slot].KnownBy, p)
		s.emit(Event{Kind: EvPeek, Player: p, Target: &t})
		s.addLog("%s 用 %s 看了自己一张牌", s.Players[p].Name, s.Pending.Label())

	case PowerSpy:
		if len(a.Targets) != 1 {
			return s, ErrBadTarget
		}
		t := a.Targets[0]
		if t.Player == p || t.Player < 0 || t.Player >= n {
			return s, ErrBadTarget
		}
		if t.Slot < 0 || t.Slot >= len(s.Players[t.Player].Hand) {
			return s, ErrBadTarget
		}
		s.Players[t.Player].Hand[t.Slot].KnownBy = addKnown(s.Players[t.Player].Hand[t.Slot].KnownBy, p)
		s.emit(Event{Kind: EvSpy, Player: p, Target: &t})
		s.addLog("%s 用 %s 看了 %s 一张牌", s.Players[p].Name, s.Pending.Label(), s.Players[t.Player].Name)

	case PowerSwap:
		if len(a.Targets) != 2 {
			return s, ErrBadTarget
		}
		t1, t2 := a.Targets[0], a.Targets[1]
		for _, t := range a.Targets {
			if t.Player < 0 || t.Player >= n {
				return s, ErrBadTarget
			}
			if t.Slot < 0 || t.Slot >= len(s.Players[t.Player].Hand) {
				return s, ErrBadTarget
			}
		}
		if t1 == t2 {
			return s, ErrBadTarget
		}
		if s.Caller >= 0 && (t1.Player == s.Caller || t2.Player == s.Caller) {
			return s, ErrLockedHand
		}
		h1 := &s.Players[t1.Player].Hand[t1.Slot]
		h2 := &s.Players[t2.Player].Hand[t2.Slot]
		c1, f1 := h1.Card, h1.FaceUp
		c2, f2 := h2.Card, h2.FaceUp
		h1.Card, h1.FaceUp = c2, f2
		h2.Card, h2.FaceUp = c1, f1
		h1.KnownBy = knownAfter(f2, p, n)
		h2.KnownBy = knownAfter(f1, p, n)
		if t1.Player == p && t2.Player == p {
			h1.KnownBy = addKnown(h1.KnownBy, p)
			h2.KnownBy = addKnown(h2.KnownBy, p)
		}
		t1c, t2c := t1, t2
		s.emit(Event{Kind: EvSwap, Player: p, Target: &t1c, Target2: &t2c})
		s.addLog("%s 用 %s 交换了两张牌", s.Players[p].Name, s.Pending.Label())
	}

	s.Discard = append(s.Discard, *s.Pending)
	s.Pending = nil
	s.Phase = PhaseTurn
	s.advance()
	return s, nil
}

func applyCallCabo(s State, p int) (State, error) {
	if s.Phase != PhaseTurn {
		return s, ErrWrongPhase
	}
	if s.Caller >= 0 {
		return s, ErrCaboLocked
	}
	n := len(s.Players)
	s.Caller = p
	s.CaboQueue = nil
	for k := 1; k < n; k++ {
		s.CaboQueue = append(s.CaboQueue, (p+k)%n)
	}
	s.emit(Event{Kind: EvCabo, Player: p})
	s.addLog("%s 喊了 CABO！其他人各再行动一次", s.Players[p].Name)
	if len(s.CaboQueue) == 0 {
		s.endRound()
		return s, nil
	}
	s.Turn = s.CaboQueue[0]
	return s, nil
}
