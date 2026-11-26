package engine

import (
	"math/rand"
	"reflect"
	"testing"
)

func mk(rank int) Card { return Card{Rank: rank} }

// testState 手工构造一个已过开局阶段的局面。deck 的最后一个元素是牌堆顶。
func testState(hands [][]Card, deck, discard []Card) State {
	n := len(hands)
	s := State{
		Caller: -1,
		Winner: -1,
		Phase:  PhaseTurn,
		Round:  1,
		Peeked: make([]bool, n),
		rng:    rand.New(rand.NewSource(5)),
	}
	for i := range hands {
		p := Player{Name: string(rune('A' + i))}
		for _, c := range hands[i] {
			p.Hand = append(p.Hand, Slot{Card: c})
		}
		s.Players = append(s.Players, p)
		s.Peeked[i] = true
	}
	s.Deck = append([]Card(nil), deck...)
	s.Discard = append([]Card(nil), discard...)
	return s
}

func totalCards(s State) int {
	n := len(s.Deck) + len(s.Discard)
	for _, p := range s.Players {
		n += len(p.Hand)
	}
	if s.Pending != nil {
		n++
	}
	return n
}

func TestNewDeck(t *testing.T) {
	d := NewDeck()
	if len(d) != 54 {
		t.Fatalf("牌数应为 54，实际 %d", len(d))
	}
	seen := map[int]bool{}
	zeros, sum := 0, 0
	for _, c := range d {
		if seen[c.ID] {
			t.Fatalf("重复的牌 ID %d", c.ID)
		}
		seen[c.ID] = true
		sum += c.Points()
		if c.Points() == 0 {
			zeros++
		}
	}
	if zeros != 3 {
		t.Fatalf("0 分牌应为 3 张，实际 %d", zeros)
	}
	// 与标准扑克牌一致：4×(1+…+12) + 3×13 = 351
	if sum != 351 {
		t.Fatalf("总点数应为 351，实际 %d", sum)
	}
}

func TestPointsAndPower(t *testing.T) {
	if mk(0).Points() != 0 || mk(13).Points() != 13 {
		t.Error("点数应等于牌面数字")
	}
	cases := map[int]Power{
		6: PowerNone, 7: PowerPeek, 8: PowerPeek,
		9: PowerSpy, 10: PowerSpy,
		11: PowerSwap, 12: PowerSwap, 13: PowerNone, 0: PowerNone,
	}
	for rank, want := range cases {
		if got := mk(rank).Power(); got != want {
			t.Errorf("%d 的技能应为 %v，实际 %v", rank, want, got)
		}
	}
}

func TestPeekPhase(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	s := NewGame([]string{"A", "B"}, []bool{false, true}, r)
	if s.Phase != PhasePeek {
		t.Fatal("开局应在看牌阶段")
	}
	if len(s.Players[0].Hand) != 4 {
		t.Fatal("每人应发 4 张")
	}

	if _, err := Apply(s, 0, Action{Type: ActPeek, Indices: []int{0}}); err != ErrBadPeekCount {
		t.Fatalf("只看 1 张应报错，实际 %v", err)
	}

	s1, err := Apply(s, 0, Action{Type: ActPeek, Indices: []int{0, 2}})
	if err != nil {
		t.Fatal(err)
	}
	if !s1.Players[0].Hand[0].KnownByPlayer(0) || !s1.Players[0].Hand[2].KnownByPlayer(0) {
		t.Fatal("看过第 0、2 张应记为已知")
	}
	if s1.Players[0].Hand[1].KnownByPlayer(0) {
		t.Fatal("没看的第 1 张不应为已知")
	}

	s2, err := Apply(s1, 1, Action{Type: ActPeek, Indices: []int{0, 1}})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Phase != PhaseTurn {
		t.Fatalf("所有人看完后应进入回合阶段，实际 %v", s2.Phase)
	}
}

func TestDrawAndPlace(t *testing.T) {
	s := testState(
		[][]Card{{mk(3), mk(4), mk(5), mk(6)}, {mk(9), mk(9), mk(9), mk(9)}},
		[]Card{mk(1), mk(2)},
		nil,
	)
	s1, err := Apply(s, 0, Action{Type: ActDrawDeck})
	if err != nil {
		t.Fatal(err)
	}
	if s1.Pending == nil || s1.Pending.Rank != 2 {
		t.Fatal("应摸到牌堆顶的 2")
	}
	if s1.Phase != PhasePlace {
		t.Fatal("摸牌后应进入安置阶段")
	}
	if len(s1.Events) == 0 || s1.Events[len(s1.Events)-1].Kind != EvDraw {
		t.Fatal("摸牌应产生 EvDraw 事件")
	}

	s2, err := Apply(s1, 0, Action{Type: ActPlace, Indices: []int{3}})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Players[0].Hand[3].Card.Rank != 2 {
		t.Fatal("第 3 位应换成 2")
	}
	if len(s2.Discard) != 1 || s2.Discard[0].Rank != 6 {
		t.Fatal("被换掉的 6 应进弃牌堆")
	}
	if s2.Phase != PhaseTurn || s2.Turn != 1 {
		t.Fatal("应轮到下家")
	}
	if s2.Events[len(s2.Events)-1].Kind != EvPlace {
		t.Fatal("换牌应产生 EvPlace 事件")
	}
}

func TestTakeDiscardMulti(t *testing.T) {
	s := testState(
		[][]Card{{mk(3), mk(3), mk(4), mk(5)}, {mk(9), mk(9), mk(9), mk(9)}},
		[]Card{mk(1), mk(2)},
		[]Card{mk(7), mk(2)},
	)
	ns, err := Apply(s, 0, Action{Type: ActTakeDiscard, Indices: []int{0, 1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ns.Players[0].Hand) != 3 {
		t.Fatalf("1 换 2 后手牌应剩 3 张，实际 %d", len(ns.Players[0].Hand))
	}
	if ns.Players[0].Hand[0].Card.Rank != 2 {
		t.Fatal("换入的 2 应落在被换出的第一个位置")
	}
	if !ns.Players[0].Hand[0].FaceUp {
		t.Fatal("从弃牌堆拿的牌应保持明牌")
	}
	if ns.Players[0].Hand[1].Card.Rank != 4 || ns.Players[0].Hand[2].Card.Rank != 5 {
		t.Fatal("其余手牌顺序应保持")
	}
	if len(ns.Discard) != 3 {
		t.Fatalf("换出的两张 3 应进弃牌堆，弃牌堆应有 3 张，实际 %d", len(ns.Discard))
	}
	if ns.Events[len(ns.Events)-1].Kind != EvTake {
		t.Fatal("应产生 EvTake 事件")
	}
}

func TestTakeDiscardWrongPenalty(t *testing.T) {
	s := testState(
		[][]Card{{mk(3), mk(4), mk(4), mk(5)}, {mk(9), mk(9), mk(9), mk(9)}},
		[]Card{mk(1), mk(2)},
		[]Card{mk(7), mk(2)},
	)
	ns, err := Apply(s, 0, Action{Type: ActTakeDiscard, Indices: []int{0, 1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ns.Players[0].Hand) != 5 {
		t.Fatalf("翻错后手牌应变成 5 张，实际 %d", len(ns.Players[0].Hand))
	}
	if !ns.Players[0].Hand[0].FaceUp || !ns.Players[0].Hand[1].FaceUp {
		t.Fatal("翻错的牌应被亮出")
	}
	if ns.Players[0].Hand[4].Card.Rank != 2 {
		t.Fatal("弃牌堆顶的 2 应被收进手里")
	}
	if ns.Events[len(ns.Events)-1].Kind != EvPenalty {
		t.Fatal("应产生 EvPenalty 事件")
	}
}

func TestTakeDiscardThreePenalty(t *testing.T) {
	s := testState(
		[][]Card{{mk(3), mk(4), mk(4), mk(5)}, {mk(9), mk(9), mk(9), mk(9)}},
		[]Card{mk(1), mk(8)},
		[]Card{mk(7), mk(2)},
	)
	ns, err := Apply(s, 0, Action{Type: ActTakeDiscard, Indices: []int{0, 1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ns.Players[0].Hand) != 6 {
		t.Fatalf("换 3 张失败应额外罚抽 1 张，手牌应为 6，实际 %d", len(ns.Players[0].Hand))
	}
	if len(ns.Deck) != 1 {
		t.Fatalf("罚抽后牌堆应剩 1 张，实际 %d", len(ns.Deck))
	}
}

func TestPowerPeekAndSpy(t *testing.T) {
	s := testState(
		[][]Card{{mk(7), mk(4), mk(4), mk(5)}, {mk(9), mk(9), mk(9), mk(9)}},
		[]Card{mk(1), mk(7)},
		nil,
	)
	s1, _ := Apply(s, 0, Action{Type: ActDrawDeck})
	if s1.Pending.Power() != PowerPeek {
		t.Fatal("应摸到 7（透视）")
	}
	if _, err := Apply(s1, 0, Action{Type: ActUsePower, Targets: []Target{{Player: 1, Slot: 0}}}); err != ErrBadTarget {
		t.Fatalf("透视不能指向对手，应报错，实际 %v", err)
	}
	s2, err := Apply(s1, 0, Action{Type: ActUsePower, Targets: []Target{{Player: 0, Slot: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if !s2.Players[0].Hand[1].KnownByPlayer(0) {
		t.Fatal("透视后该牌应为已知")
	}
	if s2.Phase != PhaseTurn || s2.Turn != 1 {
		t.Fatal("用完技能应结束回合")
	}
}

func TestSwapAndCallerLock(t *testing.T) {
	s := testState(
		[][]Card{{mk(12), mk(4), mk(4), mk(5)}, {mk(1), mk(9), mk(9), mk(9)}},
		[]Card{mk(1), mk(12)},
		nil,
	)
	s1, _ := Apply(s, 0, Action{Type: ActDrawDeck})
	s2, err := Apply(s1, 0, Action{Type: ActUsePower, Targets: []Target{
		{Player: 0, Slot: 0}, {Player: 1, Slot: 0},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Players[0].Hand[0].Card.Rank != 1 {
		t.Fatal("自己应换成对手的 1")
	}
	if s2.Players[1].Hand[0].Card.Rank != 12 {
		t.Fatal("对手应拿到 12")
	}
	if s2.Events[len(s2.Events)-1].Kind != EvSwap {
		t.Fatal("应产生 EvSwap 事件")
	}
}

func TestCallCaboScoring(t *testing.T) {
	s := testState(
		[][]Card{{mk(1), mk(1), mk(1), mk(1)}, {mk(5), mk(5), mk(5), mk(5)}, {mk(6), mk(6), mk(6), mk(6)}},
		[]Card{mk(2), mk(3)},
		nil,
	)
	s1, err := Apply(s, 0, Action{Type: ActCallCabo})
	if err != nil {
		t.Fatal(err)
	}
	if s1.Caller != 0 || s1.Turn != 1 {
		t.Fatal("喊完 CABO 应轮到下家")
	}
	if s1.Events[len(s1.Events)-1].Kind != EvCabo {
		t.Fatal("应产生 EvCabo 事件")
	}

	s2, _ := Apply(s1, 1, Action{Type: ActDrawDeck})
	s2, _ = Apply(s2, 1, Action{Type: ActDiscardPending})
	if s2.Turn != 2 {
		t.Fatalf("应轮到第三家，实际 %d", s2.Turn)
	}
	s3, _ := Apply(s2, 2, Action{Type: ActDrawDeck})
	s3, _ = Apply(s3, 2, Action{Type: ActDiscardPending})

	if s3.Phase != PhaseRoundEnd {
		t.Fatalf("结算后应进入本轮结束，实际 %v", s3.Phase)
	}
	if s3.Players[0].Score != 0 {
		t.Fatalf("喊 CABO 且最低应得 0 分，实际 %d", s3.Players[0].Score)
	}
	if s3.Players[1].Score != 20 || s3.Players[2].Score != 24 {
		t.Fatalf("其他玩家应按点数计分，实际 %d / %d", s3.Players[1].Score, s3.Players[2].Score)
	}
	hasScore := false
	for _, e := range s3.Events {
		if e.Kind == EvScore {
			hasScore = true
		}
	}
	if !hasScore {
		t.Fatal("结算应产生 EvScore 事件")
	}
}

func TestCallCaboWrongPenalty(t *testing.T) {
	s := testState(
		[][]Card{{mk(5), mk(5), mk(5), mk(5)}, {mk(1), mk(1), mk(1), mk(1)}, {mk(6), mk(6), mk(6), mk(6)}},
		[]Card{mk(2), mk(3)},
		nil,
	)
	s1, _ := Apply(s, 0, Action{Type: ActCallCabo})
	s2, _ := Apply(s1, 1, Action{Type: ActDrawDeck})
	s2, _ = Apply(s2, 1, Action{Type: ActDiscardPending})
	s3, _ := Apply(s2, 2, Action{Type: ActDrawDeck})
	s3, _ = Apply(s3, 2, Action{Type: ActDiscardPending})

	if s3.Players[0].Score != 30 {
		t.Fatalf("喊错应罚点数 +10 = 30，实际 %d", s3.Players[0].Score)
	}
	if s3.Players[1].Score != 4 {
		t.Fatalf("第二家应为 4 分，实际 %d", s3.Players[1].Score)
	}
}

func TestDeckExhaustionReshufflesDiscard(t *testing.T) {
	s := testState(
		[][]Card{{mk(3), mk(4), mk(5), mk(6)}, {mk(9), mk(9), mk(9), mk(9)}},
		nil,
		[]Card{mk(1), mk(2), mk(3), mk(7), mk(11)},
	)
	before := map[int]int{}
	for _, c := range s.Discard {
		before[c.Rank]++
	}

	ns, err := Apply(s, 0, Action{Type: ActDrawDeck})
	if err != nil {
		t.Fatal(err)
	}
	if ns.Phase != PhasePlace || ns.Pending == nil {
		t.Fatalf("牌堆摸空应洗弃牌堆继续，不该结算（phase=%v）", ns.Phase)
	}
	if ns.Reshuffle != 1 {
		t.Fatalf("应记录洗牌 1 次，实际 %d", ns.Reshuffle)
	}
	// 5 张弃牌：洗完留 1 张明牌，剩下 4 张进牌堆，被摸走 1 张后牌堆剩 3
	if len(ns.Deck) != 3 {
		t.Fatalf("牌堆应有 3 张，实际 %d", len(ns.Deck))
	}
	if len(ns.Discard) != 1 {
		t.Fatalf("弃牌堆应只留 1 张明牌，实际 %d", len(ns.Discard))
	}
	after := map[int]int{ns.Pending.Rank: 1, ns.Discard[0].Rank: 1}
	for _, c := range ns.Deck {
		after[c.Rank]++
	}
	for rank, n := range before {
		if after[rank] != n {
			t.Fatalf("洗回来的牌应当只是原来那 %d 张弃牌，%d 点对不上（%d 张）", len(before), rank, after[rank])
		}
	}
	var sawShuffle, sawDraw bool
	for _, e := range ns.Events {
		if e.Kind == EvReshuffle {
			sawShuffle = true
		}
		if e.Kind == EvDraw {
			sawDraw = true
		}
	}
	if !sawShuffle || !sawDraw {
		t.Fatalf("应同时产生 EvReshuffle 与 EvDraw，实际 %v / %v", sawShuffle, sawDraw)
	}
}

func TestBothPilesEmptyEndsRound(t *testing.T) {
	s := testState(
		[][]Card{{mk(3), mk(4), mk(5), mk(6)}, {mk(9), mk(9), mk(9), mk(9)}},
		nil,
		nil,
	)
	ns, err := Apply(s, 0, Action{Type: ActDrawDeck})
	if err != nil {
		t.Fatal(err)
	}
	if ns.Phase != PhaseRoundEnd {
		t.Fatalf("牌堆和弃牌堆都空了才应结束本轮，实际 %v", ns.Phase)
	}
}

func TestReshuffleKeepsDiscardPileDrawing(t *testing.T) {
	// 弃牌堆只剩一张时整叠洗走，不能出现"牌堆空、弃牌堆一张也拿不到"的死局
	s := testState(
		[][]Card{{mk(3), mk(4), mk(5), mk(6)}, {mk(9), mk(9), mk(9), mk(9)}},
		nil,
		[]Card{mk(2)},
	)
	ns, err := Apply(s, 0, Action{Type: ActDrawDeck})
	if err != nil {
		t.Fatal(err)
	}
	if ns.Pending == nil || ns.Pending.Rank != 2 {
		t.Fatal("最后一张弃牌应被洗进牌堆并摸到")
	}
	if len(ns.Deck) != 0 || len(ns.Discard) != 0 {
		t.Fatalf("洗完应是空堆，实际牌堆 %d / 弃牌堆 %d", len(ns.Deck), len(ns.Discard))
	}
}

func TestViewDoesNotLeak(t *testing.T) {
	s := testState(
		[][]Card{{mk(3), mk(4), mk(5), mk(6)}, {mk(9), mk(9), mk(9), mk(9)}},
		[]Card{mk(1), mk(2)},
		nil,
	)
	v := ViewFor(s, 0)
	if v.Players[1].Hand[0].Visible || v.Players[1].Hand[0].Card != nil {
		t.Fatal("对手未公开的牌不应可见，也不应带牌面数据")
	}
	if len(v.Players[0].Hand) != 4 {
		t.Fatal("自己的手牌数量应正确")
	}
	if v.Players[0].Hand[0].Card != nil {
		t.Fatal("自己没看过的牌也不应带牌面（靠记忆）")
	}
}

func TestCardConservationOverFullRound(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	s := NewGame([]string{"A", "B", "C"}, []bool{false, true, true}, r)
	if totalCards(s) != 54 {
		t.Fatalf("开局总牌数应为 54，实际 %d", totalCards(s))
	}

	for i := 0; i < 3; i++ {
		var err error
		s, err = Apply(s, i, Action{Type: ActPeek, Indices: []int{0, 1}})
		if err != nil {
			t.Fatal(err)
		}
		if totalCards(s) != 54 {
			t.Fatalf("看牌后牌数不守恒：%d", totalCards(s))
		}
	}

	steps := 0
	for s.Phase != PhaseRoundEnd && s.Phase != PhaseGameOver {
		steps++
		if steps > 500 {
			t.Fatal("回合没有正常结束，可能死循环")
		}
		p := s.Turn
		var err error
		switch {
		case s.Phase == PhasePlace:
			s, err = Apply(s, p, Action{Type: ActDiscardPending})
		case s.Reshuffle >= 2 && s.Caller < 0:
			// 洗过两轮还摸得下去，剩下的只能靠人喊 CABO 收尾
			s, err = Apply(s, p, Action{Type: ActCallCabo})
		default:
			s, err = Apply(s, p, Action{Type: ActDrawDeck})
		}
		if err != nil {
			t.Fatalf("第 %d 步出错：%v", steps, err)
		}
		if got := totalCards(s); got != 54 {
			t.Fatalf("第 %d 步牌数不守恒：%d", steps, got)
		}
	}
	if s.Reshuffle < 2 {
		t.Fatalf("牌堆摸空应把弃牌堆洗回来继续，实际只洗了 %d 次（%d 步）", s.Reshuffle, steps)
	}
}

func TestCannotActOutOfTurn(t *testing.T) {
	s := testState(
		[][]Card{{mk(3), mk(4), mk(5), mk(6)}, {mk(9), mk(9), mk(9), mk(9)}},
		[]Card{mk(1), mk(2)},
		nil,
	)
	if _, err := Apply(s, 1, Action{Type: ActDrawDeck}); err != ErrNotYourTurn {
		t.Fatalf("非当前玩家行动应报错，实际 %v", err)
	}
}

func TestSpyPowerGrantsOnlyActorKnowledge(t *testing.T) {
	s := testState(
		[][]Card{{mk(3), mk(4), mk(5), mk(6)}, {mk(9), mk(10), mk(11), mk(12)}},
		[]Card{mk(1), mk(9)},
		nil,
	)
	s1, err := Apply(s, 0, Action{Type: ActDrawDeck})
	if err != nil {
		t.Fatal(err)
	}
	s2, err := Apply(s1, 0, Action{Type: ActUsePower, Targets: []Target{{Player: 1, Slot: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	if !s2.Players[1].Hand[2].KnownByPlayer(0) {
		t.Fatal("Spy 成功后行动者应知道目标牌")
	}
	if s2.Players[1].Hand[2].KnownByPlayer(1) {
		t.Fatal("Spy 不应把对手原本不知道的牌标记为已知")
	}
	if got := s2.Events[len(s2.Events)-1]; got.Kind != EvSpy || got.Target == nil || *got.Target != (Target{Player: 1, Slot: 2}) {
		t.Fatalf("应产生带目标的 EvSpy 事件，实际 %+v", got)
	}
}

func TestCallerHandCannotBeSwapped(t *testing.T) {
	s := testState(
		[][]Card{{mk(1), mk(2), mk(3), mk(4)}, {mk(5), mk(6), mk(7), mk(8)}},
		[]Card{mk(1), mk(12)},
		nil,
	)
	s.Caller = 0
	s.Turn = 1
	s1, err := Apply(s, 1, Action{Type: ActDrawDeck})
	if err != nil {
		t.Fatal(err)
	}
	before := s1.Clone()
	_, err = Apply(s1, 1, Action{Type: ActUsePower, Targets: []Target{
		{Player: 0, Slot: 0}, {Player: 1, Slot: 0},
	}})
	if err != ErrLockedHand {
		t.Fatalf("CABO 喊者的手牌应锁定，实际 %v", err)
	}
	if !reflect.DeepEqual(s1, before) {
		t.Fatal("失败的 Swap 不应修改原状态")
	}
}

func TestExactlyHundredResetsOnlyOnce(t *testing.T) {
	s := testState(
		[][]Card{{mk(2), mk(2), mk(3), mk(3)}, {mk(1), mk(1), mk(1), mk(1)}},
		nil,
		nil,
	)
	s.Players[0].Score = 90
	s.endRound()
	if s.Players[0].Score != 50 || !s.Players[0].ResetUsed {
		t.Fatalf("恰好 100 分应重置为 50 并标记已使用，实际 %+v", s.Players[0])
	}
	if !hasEventKind(s.Events, EvReset, 0) {
		t.Fatal("100 分重置应产生 EvReset 事件")
	}

	s = testState(
		[][]Card{{mk(13), mk(13), mk(12), mk(12)}, {mk(1), mk(1), mk(1), mk(1)}},
		nil,
		nil,
	)
	s.Players[0].Score = 50
	s.Players[0].ResetUsed = true
	s.endRound()
	if s.Players[0].Score != 100 {
		t.Fatalf("每场仅一次重置，第二次恰好 100 应保留，实际 %d", s.Players[0].Score)
	}
	if hasEventKind(s.Events, EvReset, 0) {
		t.Fatal("已使用过重置时不应再次产生 EvReset")
	}
}

func TestFinalTieBreaksByPlayerIndex(t *testing.T) {
	s := testState(
		[][]Card{{mk(1), mk(1), mk(1), mk(1)}, {mk(1), mk(1), mk(1), mk(1)}},
		nil,
		nil,
	)
	s.Players[0].Score = 100
	s.Players[1].Score = 100
	s.endRound()
	if s.Phase != PhaseGameOver || s.Winner != 0 {
		t.Fatalf("最终同分应选择下标较小的玩家，实际 phase=%s winner=%d", s.Phase, s.Winner)
	}
}

func TestSwapMovesKnowledgeWithCards(t *testing.T) {
	s := testState(
		[][]Card{{mk(12), mk(4), mk(4), mk(5)}, {mk(1), mk(9), mk(9), mk(9)}},
		[]Card{mk(1), mk(12)},
		nil,
	)
	s.Players[0].Hand[0].KnownBy = []int{0}
	s.Players[1].Hand[0].KnownBy = []int{1}
	s1, err := Apply(s, 0, Action{Type: ActDrawDeck})
	if err != nil {
		t.Fatal(err)
	}
	s2, err := Apply(s1, 0, Action{Type: ActUsePower, Targets: []Target{
		{Player: 0, Slot: 0}, {Player: 1, Slot: 0},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !s2.Players[0].Hand[0].KnownByPlayer(0) || !s2.Players[1].Hand[0].KnownByPlayer(0) {
		t.Fatal("行动者应知道交换后的两张牌")
	}
	if s2.Players[0].Hand[0].KnownByPlayer(1) || s2.Players[1].Hand[0].KnownByPlayer(1) {
		t.Fatal("对手对原位置的记忆应随牌移动而失效")
	}
}

func TestPowerName(t *testing.T) {
	cases := map[Power]string{
		PowerNone: "",
		PowerPeek: "透视",
		PowerSpy:  "窥探",
		PowerSwap: "置换",
	}
	for power, want := range cases {
		if got := power.Name(); got != want {
			t.Errorf("Power %d name = %q, want %q", power, got, want)
		}
	}
}

func TestNextRoundStartsFreshPeekPhase(t *testing.T) {
	s := testState(
		[][]Card{{mk(1), mk(1), mk(1), mk(1)}, {mk(2), mk(2), mk(2), mk(2)}},
		nil,
		nil,
	)
	s.Phase = PhaseRoundEnd
	s.Round = 1
	s.Players[0].Score = 12
	s.Players[0].ResetUsed = true
	next := NextRound(s)
	if next.Round != 2 || next.Phase != PhasePeek || next.Caller != -1 || next.Winner != -1 {
		t.Fatalf("unexpected next round: %+v", next)
	}
	if next.Players[0].Score != 12 || !next.Players[0].ResetUsed {
		t.Fatal("next round should preserve career score and reset usage")
	}
	for i, player := range next.Players {
		if len(player.Hand) != 4 || player.RoundPts != 0 || next.Peeked[i] {
			t.Fatalf("player %d was not reset: %+v", i, player)
		}
	}
	if totalCards(next) != 54 {
		t.Fatalf("next round card count = %d", totalCards(next))
	}
}

func TestViewForExposesOnlyPublicAndKnownCards(t *testing.T) {
	pending := mk(9)
	s := testState(
		[][]Card{
			{mk(1), mk(2), mk(3), mk(4)},
			{mk(5), mk(6), mk(7), mk(8)},
		},
		[]Card{mk(10), mk(11)},
		[]Card{mk(0), mk(1), mk(2), mk(3), mk(4), mk(5), mk(6), mk(7)},
	)
	s.Phase = PhasePlace
	s.Caller = 0
	s.Pending = &pending
	s.Players[0].Hand[0].FaceUp = true
	s.Players[0].Hand[1].KnownBy = []int{0}
	s.Players[1].Hand[0].KnownBy = []int{0}

	view := ViewFor(s, 0)
	if view.DiscardTop == nil || view.DiscardTop.Points != 7 || len(view.DiscardUnder) != 6 {
		t.Fatalf("unexpected discard view: top=%+v under=%d", view.DiscardTop, len(view.DiscardUnder))
	}
	if view.Pending == nil || view.Pending.Points != 9 {
		t.Fatalf("pending card should be visible to the actor: %+v", view.Pending)
	}
	if !view.Can.DiscardPending || !view.Can.UsePower {
		t.Fatalf("place capabilities = %+v", view.Can)
	}
	if !view.Players[0].IsTurn || !view.Players[0].IsCaller || !view.Players[0].Hand[0].Visible || !view.Players[0].Hand[1].Visible {
		t.Fatalf("player view = %+v", view.Players[0])
	}
	if view.Players[0].Hand[2].Visible || !view.Players[1].Hand[0].Visible {
		t.Fatal("known-by flags were not respected")
	}
	if ViewFor(s, 1).Pending != nil {
		t.Fatal("another player must not see the pending card")
	}

	s.Phase = PhaseGameOver
	if !ViewFor(s, 0).GameOver {
		t.Fatal("gameover phase should be reflected in the view")
	}
}

func TestApplyRejectsInvalidStateAndPlayer(t *testing.T) {
	s := testState([][]Card{{mk(1)}}, nil, nil)
	if _, err := Apply(s, 1, Action{Type: ActDrawDeck}); err != ErrBadPlayer {
		t.Fatalf("bad player error = %v", err)
	}
	s.Phase = PhaseGameOver
	if _, err := Apply(s, 0, Action{Type: ActDrawDeck}); err != ErrGameOver {
		t.Fatalf("gameover error = %v", err)
	}
	s.Phase = PhaseTurn
	if _, err := Apply(s, 0, Action{Type: ActPlace}); err != ErrNoPending {
		t.Fatalf("missing pending error = %v", err)
	}
}

func hasEventKind(events []Event, kind EventKind, player int) bool {
	for _, event := range events {
		if event.Kind == kind && event.Player == player {
			return true
		}
	}
	return false
}
