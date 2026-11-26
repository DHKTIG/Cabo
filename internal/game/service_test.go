package game

import (
	"errors"
	"sync"
	"testing"

	"cabo/internal/engine"
	"cabo/internal/storage"
)

type memoryStore struct {
	mu       sync.Mutex
	snapshot storage.StatsSnapshot
	fail     bool
	updates  int
	writes   int
}

func (s *memoryStore) LoadStats() (storage.StatsSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := storage.StatsSnapshot{Stats: s.snapshot.Stats}
	if len(s.snapshot.Settlements) > 0 {
		copy.Settlements = make(map[string]struct{}, len(s.snapshot.Settlements))
		for id := range s.snapshot.Settlements {
			copy.Settlements[id] = struct{}{}
		}
	}
	return copy, nil
}

func (s *memoryStore) UpdateStats(update func(storage.StatsSnapshot) (storage.StatsSnapshot, error)) (storage.StatsSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates++
	if s.fail {
		return s.snapshot, errors.New("storage unavailable")
	}
	updated, err := update(s.snapshot)
	if err != nil {
		return s.snapshot, err
	}
	s.snapshot = updated
	s.writes++
	return updated, nil
}

func newTestService() (*Service, *memoryStore) {
	store := &memoryStore{snapshot: storage.StatsSnapshot{Stats: storage.EmptyStats()}}
	return New(store, 42), store
}

func TestNotReadyMethods(t *testing.T) {
	service, _ := newTestService()
	if _, ready := service.Snapshot(); ready {
		t.Fatal("new service should not be ready")
	}
	if _, err := service.Apply(engine.Action{Type: engine.ActDrawDeck}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("apply error = %v", err)
	}
	if _, err := service.AdvanceAI(); !errors.Is(err, ErrNotReady) {
		t.Fatalf("AI error = %v", err)
	}
}

func TestNewGameClampsInputs(t *testing.T) {
	service, _ := newTestService()
	view, err := service.NewGame("", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Players) != 2 || view.Players[0].Name != "你" {
		t.Fatalf("default new game = %+v", view.Players)
	}
	view, err = service.NewGame("玩家", 99, 99)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Players) != 4 {
		t.Fatalf("clamped AI count = %d", len(view.Players))
	}
}

func TestNewGameAndSnapshot(t *testing.T) {
	service, _ := newTestService()
	view, err := service.NewGame("玩家", 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if view.Phase != engine.PhasePeek || len(view.Players) != 3 {
		t.Fatalf("unexpected new game: %+v", view)
	}
	snapshot, ready := service.Snapshot()
	if !ready || snapshot.Phase != engine.PhasePeek {
		t.Fatalf("snapshot not ready: %+v, %v", snapshot, ready)
	}
	if len(snapshot.Events) != 0 {
		t.Fatalf("snapshot should not replay consumed events: %+v", snapshot.Events)
	}

	view, err = service.Apply(engine.Action{Type: engine.ActPeek, Indices: []int{0, 1}})
	if err != nil {
		t.Fatal(err)
	}
	if view.Phase != engine.PhaseTurn {
		t.Fatalf("peek should start the round: %+v", view)
	}
	foundPeek := false
	for _, event := range view.Events {
		if event.Kind == engine.EvPeek && event.Player == 0 {
			foundPeek = true
		}
	}
	if !foundPeek {
		t.Fatal("mutation response should include the peek event")
	}
	snapshot, _ = service.Snapshot()
	if len(snapshot.Events) != 0 {
		t.Fatal("snapshot should not consume or replay mutation events")
	}
}

func TestSnapshotDoesNotConsumeEvents(t *testing.T) {
	service, _ := newTestService()
	service.ready = true
	service.state = engine.State{Events: []engine.Event{{Kind: engine.EvDraw, Player: 0}}}
	for i := 0; i < 2; i++ {
		view, ready := service.Snapshot()
		if !ready || len(view.Events) != 1 {
			t.Fatalf("snapshot %d lost events: %+v", i, view.Events)
		}
	}
	if len(service.state.Events) != 1 {
		t.Fatal("snapshot mutated the stored event queue")
	}
}

func TestAdvanceAIAppliesOneAction(t *testing.T) {
	service, _ := newTestService()
	if _, err := service.NewGame("玩家", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Apply(engine.Action{Type: engine.ActPeek, Indices: []int{0, 1}}); err != nil {
		t.Fatal(err)
	}
	service.state.Turn = 1
	view, err := service.AdvanceAI()
	if err != nil {
		t.Fatal(err)
	}
	if view.Phase != engine.PhasePlace || view.Turn != 1 {
		t.Fatalf("AI should draw one card: %+v", view)
	}
	service.state.Phase = engine.PhaseRoundEnd
	if _, err := service.AdvanceAI(); err != nil {
		t.Fatalf("AI should be a no-op outside a turn: %v", err)
	}
}

func TestNextRoundStartsPeekPhase(t *testing.T) {
	service, _ := newTestService()
	if _, err := service.NewGame("玩家", 1, 1); err != nil {
		t.Fatal(err)
	}
	service.state.Phase = engine.PhaseRoundEnd
	view, err := service.NextRound()
	if err != nil {
		t.Fatal(err)
	}
	if view.Round != 2 || view.Phase != engine.PhasePeek {
		t.Fatalf("unexpected next round view: %+v", view)
	}
}

func TestRoundSettlementIsIdempotent(t *testing.T) {
	service, store := newTestService()
	service.ready = true
	service.gameID = "game-1"
	service.state = engine.State{
		Phase:  engine.PhaseRoundEnd,
		Round:  1,
		Caller: 0,
		Players: []engine.Player{
			{RoundPts: 0},
		},
	}
	service.roundBest = 3
	service.cabos = 1
	service.settleIfNeeded()
	service.settleIfNeeded()
	stats, err := service.Stats()
	if err != nil {
		t.Fatal(err)
	}
	want := storage.Stats{BestRound: 0, BestStreak: 3, Rounds: 1, Perfect: 1, Cabos: 1, CabosWon: 1}
	if stats != want {
		t.Fatalf("stats = %+v, want %+v", stats, want)
	}
	if store.writes != 1 {
		t.Fatalf("duplicate settlement caused %d writes", store.writes)
	}
	if _, err := service.Stats(); err != nil {
		t.Fatal(err)
	}
	if store.writes != 1 {
		t.Fatal("reading stats rewrote the settlement")
	}
}

func TestSettlementRetriesAfterStoreFailure(t *testing.T) {
	service, store := newTestService()
	service.ready = true
	service.gameID = "game-1"
	service.state = engine.State{
		Phase:  engine.PhaseRoundEnd,
		Round:  1,
		Caller: -1,
		Players: []engine.Player{
			{RoundPts: 8},
		},
	}
	store.fail = true
	service.settleIfNeeded()
	if service.pending == nil {
		t.Fatal("failed settlement should remain pending")
	}
	if _, err := service.Stats(); err == nil {
		t.Fatal("expected pending storage error")
	}
	store.fail = false
	stats, err := service.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Rounds != 1 || stats.BestRound != 8 || service.pending != nil {
		t.Fatalf("retry did not finish settlement: %+v", stats)
	}
}

func TestNewGameFlushesPendingSettlement(t *testing.T) {
	service, store := newTestService()
	service.pending = &roundResult{id: "old-game:1", bestRound: 6, bestStreak: 2, perfect: 1}
	store.fail = true
	if _, err := service.NewGame("玩家", 1, 1); err == nil {
		t.Fatal("new game should fail while an old settlement cannot be saved")
	}
	if service.pending == nil || service.ready {
		t.Fatal("failed flush discarded pending progress or started a new game")
	}
	store.fail = false
	if _, err := service.NewGame("玩家", 1, 1); err != nil {
		t.Fatal(err)
	}
	stats, err := service.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Rounds != 1 || stats.BestRound != 6 {
		t.Fatalf("old settlement was lost: %+v", stats)
	}
}

func TestNextRoundRejectsWrongPhase(t *testing.T) {
	service, _ := newTestService()
	if _, err := service.NextRound(); !errors.Is(err, ErrNotReady) {
		t.Fatalf("not-ready error = %v", err)
	}
	service.ready = true
	service.state.Phase = engine.PhaseTurn
	if _, err := service.NextRound(); !errors.Is(err, engine.ErrWrongPhase) {
		t.Fatalf("wrong-phase error = %v", err)
	}
}
