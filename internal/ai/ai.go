package ai

import (
	"math/rand"

	"cabo/internal/engine"
)

// Level 难度档位。区别全在决策参数上，不靠作弊（不看暗牌、不改牌堆）。
type Level int

const (
	Easy   Level = 0
	Normal Level = 1
	Hard   Level = 2
)

// params 一个档位的全部决策旋钮。想调节奏只改这张表。
//
//	callAt    四张全知道时，总点数不高于这个就喊 CABO
//	callLate  牌堆剩不到 8 张时放宽到（越接近收尾越该赌）
//	gambleAt  只剩一张未知时，已知部分不高于这个才考虑赌；-1 表示永不赌
//	gambleP   满足赌的条件时真的喊的概率（留一点不确定性，别像机器）
//	swapGap   「我知道对手那张比我这张低」时，净收益至少多少才换
//	disruptMin 干扰置换时自己这张牌至少多少点——高点数塞给别人才值
//	disruptP   尝试干扰置换的概率
//	grabMax   弃牌堆顶不高于这个点、且手上还有未知牌时，直接吃进来换掉一张未知
//	targetLeader 窥探与干扰是否优先针对总分最低的人
type params struct {
	callAt       int
	callLate     int
	gambleAt     int
	gambleP      float64
	swapGap      int
	disruptMin   int
	disruptP     float64
	grabMax      int
	targetLeader bool
}

var levelParams = map[Level]params{
	// 轻松：几乎不赌、不换、不干扰，回合偏长但玩家压力小
	Easy: {callAt: 7, callLate: 11, gambleAt: -1, gambleP: 0, swapGap: 6, disruptMin: 13, disruptP: 0.05, grabMax: 2, targetLeader: false},
	// 标准：会赌一张未知、会干扰领先者，节奏接近真人桌
	Normal: {callAt: 10, callLate: 14, gambleAt: 7, gambleP: 0.6, swapGap: 4, disruptMin: 10, disruptP: 0.45, grabMax: 4, targetLeader: true},
	// 较真：更敢赌、更爱换，盯着领先者打
	Hard: {callAt: 13, callLate: 17, gambleAt: 10, gambleP: 0.85, swapGap: 3, disruptMin: 10, disruptP: 0.6, grabMax: 5, targetLeader: true},
}

func paramsOf(l Level) params {
	if p, ok := levelParams[l]; ok {
		return p
	}
	return levelParams[Normal]
}

// Decide 给出 AI 在当前局面的下一步动作。
func Decide(s engine.State, p int, r *rand.Rand, l Level) engine.Action {
	pr := paramsOf(l)
	switch s.Phase {
	case engine.PhasePeek:
		perm := r.Perm(4)
		return engine.Action{Type: engine.ActPeek, Indices: []int{perm[0], perm[1]}}
	case engine.PhaseTurn:
		return decideTurn(s, p, r, pr)
	case engine.PhasePlace:
		return decidePlace(s, p, r, pr)
	}
	return engine.Action{Type: engine.ActDrawDeck}
}

func decideTurn(s engine.State, p int, r *rand.Rand, pr params) engine.Action {
	hand := s.Players[p].Hand
	known, unknown := 0, 0
	for _, sl := range hand {
		if sl.KnownByPlayer(p) {
			known += sl.Card.Points()
		} else {
			unknown++
		}
	}

	if s.Caller < 0 {
		limit := pr.callAt
		// 牌堆见底或已经洗过一次，说明这轮拖长了：放宽喊话门槛，别让回合无限走下去
		if len(s.Deck) < 8 || s.Reshuffle > 0 {
			limit = pr.callLate
		}
		switch {
		case unknown == 0 && known <= limit:
			return engine.Action{Type: engine.ActCallCabo}
		case unknown == 1 && pr.gambleAt >= 0 && known <= pr.gambleAt && r.Float64() < pr.gambleP:
			// 剩一张未知就喊，是 CABO 最有人味的一手：可能赌赢也可能 +10
			return engine.Action{Type: engine.ActCallCabo}
		}
	}

	if len(s.Discard) > 0 {
		topVal := s.Discard[len(s.Discard)-1].Points()

		// 1 换多：手上有两张已知同点的高牌，一次清掉
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

		// 吃进一张明牌低牌，顺手把一张未知牌换成已知的——期望上稳赚，也推进回合
		if topVal <= pr.grabMax && unknown > 0 {
			var unk []int
			for i, sl := range hand {
				if !sl.KnownByPlayer(p) {
					unk = append(unk, i)
				}
			}
			if len(unk) > 0 {
				return engine.Action{Type: engine.ActTakeDiscard, Indices: []int{unk[r.Intn(len(unk))]}}
			}
		}
	}

	return engine.Action{Type: engine.ActDrawDeck}
}

func decidePlace(s engine.State, p int, r *rand.Rand, pr params) engine.Action {
	pend := *s.Pending

	if pw := pend.Power(); pw != engine.PowerNone {
		if t, ok := powerTarget(s, p, pw, r, pr); ok {
			return engine.Action{Type: engine.ActUsePower, Targets: t}
		}
	}

	hand := s.Players[p].Hand
	worst, worstVal := -1, -1
	for i, sl := range hand {
		if sl.KnownByPlayer(p) && sl.Card.Points() > worstVal {
			worstVal, worst = sl.Card.Points(), i
		}
	}
	if worst >= 0 && worstVal > pend.Points() {
		return engine.Action{Type: engine.ActPlace, Indices: []int{worst}}
	}
	return engine.Action{Type: engine.ActDiscardPending}
}

func powerTarget(s engine.State, p int, pw engine.Power, r *rand.Rand, pr params) ([]engine.Target, bool) {
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
		cands := spyCandidates(s, p, pr)
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

		// 确定收益：我知道对手某张牌，且换过来净赚够多
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
		if opBest.Player >= 0 && myVal-opVal >= pr.swapGap {
			return []engine.Target{{Player: p, Slot: myBest}, opBest}, true
		}

		// 干扰：把我这张高牌塞进领先者的一张未知牌位。
		// 期望上我用 11–13 换回一张约 6.5 的未知牌，同时废掉他对那个位置的记忆——
		// 这是 CABO 里最让对方难受的一手，之前因为门槛太高几乎从不发生。
		if myVal >= pr.disruptMin && r.Float64() < pr.disruptP {
			for _, pi := range leaderOrder(s, p, pr) {
				if locked(pi) {
					continue
				}
				var unk []int
				for si, sl := range s.Players[pi].Hand {
					if !sl.KnownByPlayer(p) {
						unk = append(unk, si)
					}
				}
				if len(unk) > 0 {
					return []engine.Target{{Player: p, Slot: myBest}, {Player: pi, Slot: unk[r.Intn(len(unk))]}}, true
				}
			}
		}
		return nil, false
	}
	return nil, false
}

// leaderOrder 按总分从低到高返回对手编号；不针对领先者时退化为随机序。
func leaderOrder(s engine.State, p int, pr params) []int {
	var out []int
	for pi := range s.Players {
		if pi != p {
			out = append(out, pi)
		}
	}
	if pr.targetLeader {
		for i := 1; i < len(out); i++ {
			for j := i; j > 0 && s.Players[out[j]].Score < s.Players[out[j-1]].Score; j-- {
				out[j], out[j-1] = out[j-1], out[j]
			}
		}
	}
	return out
}

// spyCandidates 可窥探的目标。盯着领先者看，比随机看更能改变局势。
func spyCandidates(s engine.State, p int, pr params) []engine.Target {
	var out []engine.Target
	if pr.targetLeader {
		for _, pi := range leaderOrder(s, p, pr) {
			for si, sl := range s.Players[pi].Hand {
				if !sl.KnownByPlayer(p) {
					out = append(out, engine.Target{Player: pi, Slot: si})
				}
			}
			if len(out) > 0 {
				return out
			}
		}
		return out
	}
	for pi := range s.Players {
		if pi == p {
			continue
		}
		for si, sl := range s.Players[pi].Hand {
			if !sl.KnownByPlayer(p) {
				out = append(out, engine.Target{Player: pi, Slot: si})
			}
		}
	}
	return out
}
