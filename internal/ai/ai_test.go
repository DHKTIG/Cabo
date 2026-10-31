package ai

import (
	"math/rand"
	"testing"

	"cabo/internal/engine"
)

func countCards(s engine.State) int {
	n := len(s.Deck) + len(s.Discard)
	for _, p := range s.Players {
		n += len(p.Hand)
	}
	if s.Pending != nil {
		n++
	}
	return n
}

// TestAIGamesKeepCardCount 让 4 个 AI 互相对打，验证每一步都不丢牌、不出非法动作、
// 且对局必定在有限步内结束。这是发现状态管理 bug 最有效的一条断言。
func TestAIGamesKeepCardCount(t *testing.T) {
	for seed := 0; seed < 60; seed++ {
		r := rand.New(rand.NewSource(int64(seed)))
		s := engine.NewGame(
			[]string{"A", "B", "C", "D"},
			[]bool{true, true, true, true},
			r,
		)

		for step := 0; step < 8000; step++ {
			if s.Phase == engine.PhaseGameOver {
				break
			}
			if s.Phase == engine.PhaseRoundEnd {
				s = engine.NextRound(s, r)
				if got := countCards(s); got != 54 {
					t.Fatalf("seed %d 新发牌后牌数 %d", seed, got)
				}
				continue
			}

			p := s.Turn
			if s.Phase == engine.PhasePeek {
				for i := range s.Players {
					if !s.Peeked[i] {
						p = i
						break
					}
				}
			}

			a := Decide(s, p, r, Normal)
			before := countCards(s)
			ns, err := engine.Apply(s, p, a)
			if err != nil {
				t.Fatalf("seed %d step %d 玩家 %d 动作 %+v 被拒：%v（阶段 %s）",
					seed, step, p, a, err, s.Phase)
			}
			if after := countCards(ns); after != before {
				t.Fatalf("seed %d step %d 玩家 %d 动作 %+v 导致牌数 %d -> %d",
					seed, step, p, a, before, after)
			}
			for i, pl := range ns.Players {
				if len(pl.Hand) > 8 {
					t.Fatalf("seed %d 玩家 %d 手牌异常膨胀到 %d 张", seed, i, len(pl.Hand))
				}
				if len(pl.Hand) == 0 && ns.Phase != engine.PhaseRoundEnd && ns.Phase != engine.PhaseGameOver {
					t.Fatalf("seed %d 玩家 %d 手牌为空", seed, i)
				}
			}
			s = ns
		}

		if s.Phase != engine.PhaseGameOver {
			t.Fatalf("seed %d 对局没有在 8000 步内结束，停在 %s", seed, s.Phase)
		}
	}
}
