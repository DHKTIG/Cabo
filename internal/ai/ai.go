package ai

import (
	"math/rand"

	"cabo/internal/engine"
)

// Decide 给出 AI 在当前局面的下一步动作。
func Decide(s engine.State, p int, r *rand.Rand) engine.Action {
	switch s.Phase {
	case engine.PhasePeek:
		perm := r.Perm(4)
		return engine.Action{Type: engine.ActPeek, Indices: []int{perm[0], perm[1]}}
	case engine.PhaseTurn:
		return decideTurn(s, p)
	case engine.PhasePlace:
		return decidePlace(s, p, r)
	}
	return engine.Action{Type: engine.ActDrawDeck}
}

func decideTurn(s engine.State, p int) engine.Action {
	hand := s.Players[p].Hand
	total, unknown := 0, 0
	for _, sl := range hand {
		if sl.KnownByPlayer(p) {
			total += sl.Card.Points()
		} else {
			unknown++
		}
	}

	if s.Caller < 0 && unknown == 0 {
		if total <= 9 || (len(s.Deck) < 8 && total <= 13) {
			return engine.Action{Type: engine.ActCallCabo}
		}
	}

	if len(s.Discard) > 0 {
		topVal := s.Discard[len(s.Discard)-1].Points()

		// 1 换多：手上有两张已知同点的高牌
		groups := map[int][]int{}
		for i, sl := range hand {
			if sl.KnownByPlayer(p) {
				groups[sl.Card.Rank] = append(groups[sl.Card.Rank], i)
			}
		}
		bestRank, bestIdx := -1, []int(nil)
		for rank, idxs := range groups {
			if len(idxs) >= 2 && rank > topVal+1 && rank > bestRank {
				bestRank = rank
				bestIdx = idxs[:2]
			}
		}
		if bestRank > 0 {
			return engine.Action{Type: engine.ActTakeDiscard, Indices: bestIdx}
		}

		// 1 换 1：拿低牌换掉自己最高的已知牌
		worst, worstVal := -1, -1
		for i, sl := range hand {
			if sl.KnownByPlayer(p) && sl.Card.Points() > worstVal {
				worstVal, worst = sl.Card.Points(), i
			}
		}
		if worst >= 0 && topVal < worstVal-1 {
			return engine.Action{Type: engine.ActTakeDiscard, Indices: []int{worst}}
		}
	}

	return engine.Action{Type: engine.ActDrawDeck}
}

func decidePlace(s engine.State, p int, r *rand.Rand) engine.Action {
	pend := *s.Pending
	pv := pend.Points()
	hand := s.Players[p].Hand

	if pw := pend.Power(); pw != engine.PowerNone {
		if t, ok := powerTarget(s, p, pw, r); ok {
			return engine.Action{Type: engine.ActUsePower, Targets: t}
		}
	}

	worst, worstVal := -1, -1
	for i, sl := range hand {
		if sl.KnownByPlayer(p) && sl.Card.Points() > worstVal {
			worstVal, worst = sl.Card.Points(), i
		}
	}
	if worst >= 0 && worstVal > pv {
		return engine.Action{Type: engine.ActPlace, Indices: []int{worst}}
	}
	return engine.Action{Type: engine.ActDiscardPending}
}

func powerTarget(s engine.State, p int, pw engine.Power, r *rand.Rand) ([]engine.Target, bool) {
	hand := s.Players[p].Hand
	locked := func(pi int) bool { return s.Caller >= 0 && pi == s.Caller }

	switch pw {
	case engine.PowerPeek:
		var unk []int
		for i, sl := range hand {
			if !sl.KnownByPlayer(p) {
				unk = append(unk, i)
			}
		}
		if len(unk) == 0 {
			return nil, false
		}
		return []engine.Target{{Player: p, Slot: unk[r.Intn(len(unk))]}}, true

	case engine.PowerSpy:
		var cands []engine.Target
		for pi := range s.Players {
			if pi == p {
				continue
			}
			for si, sl := range s.Players[pi].Hand {
				if !sl.KnownByPlayer(p) {
					cands = append(cands, engine.Target{Player: pi, Slot: si})
				}
			}
		}
		if len(cands) == 0 {
			return nil, false
		}
		return []engine.Target{cands[r.Intn(len(cands))]}, true

	case engine.PowerSwap:
		myBest, myVal := -1, -1
		for i, sl := range hand {
			if sl.KnownByPlayer(p) && sl.Card.Points() > myVal {
				myVal, myBest = sl.Card.Points(), i
			}
		}
		if myBest < 0 {
			return nil, false
		}
		opBest, opVal := engine.Target{Player: -1}, 99
		for pi := range s.Players {
			if pi == p || locked(pi) {
				continue
			}
			for si, sl := range s.Players[pi].Hand {
				if sl.KnownByPlayer(p) && sl.Card.Points() < opVal {
					opVal, opBest = sl.Card.Points(), engine.Target{Player: pi, Slot: si}
				}
			}
		}
		if opBest.Player < 0 || myVal-opVal < 5 {
			return nil, false
		}
		return []engine.Target{{Player: p, Slot: myBest}, opBest}, true
	}
	return nil, false
}
