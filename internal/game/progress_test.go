package game

import (
	"testing"

	"cabo/internal/engine"
)

func TestPlaceStreakRequiresKnownCard(t *testing.T) {
	service, _ := newTestService()
	pending := engine.Card{Rank: 5}
	before := engine.State{
		Pending: &pending,
		Players: []engine.Player{{Hand: []engine.Slot{
			{Card: engine.Card{Rank: 10}, FaceUp: true},
			{Card: engine.Card{Rank: 10}},
		}}},
	}
	action := engine.Action{Type: engine.ActPlace, Indices: []int{0}}
	service.updateRoundProgress(0, before, before, action)
	if service.streak != 1 || service.roundBest != 1 {
		t.Fatalf("known lower placement should increment streak: %+d", service.streak)
	}
	service.updateRoundProgress(0, before, before, engine.Action{Type: engine.ActPlace, Indices: []int{1}})
	if service.streak != 1 {
		t.Fatal("unknown placement should be neutral")
	}
	pending.Rank = 12
	service.updateRoundProgress(0, before, before, action)
	if service.streak != 0 {
		t.Fatal("known higher placement should reset streak")
	}
}

func TestTakeDiscardStreakAndPenalty(t *testing.T) {
	service, _ := newTestService()
	before := engine.State{
		Discard: []engine.Card{{Rank: 5}},
		Players: []engine.Player{{Hand: []engine.Slot{
			{Card: engine.Card{Rank: 10}, KnownBy: []int{0}},
			{Card: engine.Card{Rank: 11}, FaceUp: true},
			{Card: engine.Card{Rank: 12}},
		}}},
	}
	action := engine.Action{Type: engine.ActTakeDiscard, Indices: []int{0, 1}}
	service.updateRoundProgress(0, before, before, action)
	if service.streak != 1 {
		t.Fatal("all known higher cards should increment streak")
	}
	service.updateRoundProgress(0, before, before, engine.Action{Type: engine.ActTakeDiscard, Indices: []int{0, 2}})
	if service.streak != 1 {
		t.Fatal("unknown selected card should make the attempt neutral")
	}
	before.Discard[0].Rank = 12
	service.updateRoundProgress(0, before, before, action)
	if service.streak != 0 {
		t.Fatal("non-improving known take should reset streak")
	}

	after := before
	after.Events = []engine.Event{{Kind: engine.EvPenalty, Player: 0}}
	service.streak = 4
	service.updateRoundProgress(0, before, after, action)
	if service.streak != 0 || service.mistakes != 1 {
		t.Fatalf("penalty should reset streak and count a mistake: streak=%d mistakes=%d", service.streak, service.mistakes)
	}
}

func TestAIActionsDoNotAffectPlayerStreak(t *testing.T) {
	service, _ := newTestService()
	service.streak = 2
	pending := engine.Card{Rank: 1}
	before := engine.State{
		Pending: &pending,
		Players: []engine.Player{{Hand: []engine.Slot{{Card: engine.Card{Rank: 13}, FaceUp: true}}}},
	}
	service.updateRoundProgress(1, before, before, engine.Action{Type: engine.ActPlace, Indices: []int{0}})
	if service.streak != 2 || service.roundBest != 0 {
		t.Fatalf("AI action changed player progress: streak=%d best=%d", service.streak, service.roundBest)
	}
}

func TestStreakIsCapped(t *testing.T) {
	service, _ := newTestService()
	service.setStreak(1000)
	if service.streak != 99 || service.roundBest != 99 {
		t.Fatalf("streak cap = %d, best = %d", service.streak, service.roundBest)
	}
}
