package game

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"cabo/internal/ai"
	"cabo/internal/engine"
	"cabo/internal/storage"
)

var ErrNotReady = errors.New("对局尚未开始")

type Store interface {
	LoadStats() (storage.StatsSnapshot, error)
	UpdateStats(func(storage.StatsSnapshot) (storage.StatsSnapshot, error)) (storage.StatsSnapshot, error)
}

type Service struct {
	mu      sync.Mutex
	store   Store
	state   engine.State
	rng     *rand.Rand
	ready   bool
	level   ai.Level
	gameID  string
	pending *roundResult

	streak    int
	roundBest int
	mistakes  int
	cabos     int
}

func New(store Store, seed int64) *Service {
	return &Service{
		store: store,
		rng:   rand.New(rand.NewSource(seed)),
	}
}

func (s *Service) NewGame(name string, aiCount, level int) (engine.View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.flushPending(); err != nil {
		return engine.View{}, fmt.Errorf("flush previous round: %w", err)
	}
	id, err := newGameID()
	if err != nil {
		return engine.View{}, err
	}
	if aiCount < 1 {
		aiCount = 1
	}
	if aiCount > 3 {
		aiCount = 3
	}
	if level < 0 {
		level = 0
	}
	if level > 2 {
		level = 2
	}
	if name == "" {
		name = "你"
	}

	names := []string{name}
	flags := []bool{false}
	letters := []string{"电脑 A", "电脑 B", "电脑 C"}
	for i := 0; i < aiCount; i++ {
		names = append(names, letters[i])
		flags = append(flags, true)
	}

	s.gameID = id
	s.level = ai.Level(level)
	s.state = engine.NewGame(names, flags, s.rng)
	s.ready = true
	s.pending = nil
	s.resetRoundProgress()

	for i := 1; i < len(names); i++ {
		next, err := engine.Apply(s.state, i, ai.Decide(s.state, i, s.rng, s.level))
		if err != nil {
			return engine.View{}, fmt.Errorf("AI 开局看牌: %w", err)
		}
		s.state = next
	}
	return s.consumeView(), nil
}

func (s *Service) Snapshot() (engine.View, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready {
		return engine.View{}, false
	}
	return engine.ViewFor(s.state, 0), true
}

func (s *Service) Apply(action engine.Action) (engine.View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready {
		return engine.View{}, ErrNotReady
	}
	return s.apply(0, action)
}

func (s *Service) AdvanceAI() (engine.View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready {
		return engine.View{}, ErrNotReady
	}

	if s.state.Phase == engine.PhasePeek {
		for i := 1; i < len(s.state.Players); i++ {
			if !s.state.Peeked[i] {
				return s.apply(i, ai.Decide(s.state, i, s.rng, s.level))
			}
		}
		return s.consumeView(), nil
	}
	if s.state.Phase != engine.PhaseTurn && s.state.Phase != engine.PhasePlace {
		return s.consumeView(), nil
	}
	player := s.state.Turn
	if player <= 0 || player >= len(s.state.Players) {
		return s.consumeView(), nil
	}
	return s.apply(player, ai.Decide(s.state, player, s.rng, s.level))
}

func (s *Service) NextRound() (engine.View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready {
		return engine.View{}, ErrNotReady
	}
	if s.state.Phase != engine.PhaseRoundEnd {
		return engine.View{}, engine.ErrWrongPhase
	}

	s.state = engine.NextRound(s.state)
	s.resetRoundProgress()
	for i := 1; i < len(s.state.Players); i++ {
		next, err := engine.Apply(s.state, i, ai.Decide(s.state, i, s.rng, s.level))
		if err != nil {
			return engine.View{}, fmt.Errorf("AI 新一轮看牌: %w", err)
		}
		s.state = next
	}
	return s.consumeView(), nil
}

func (s *Service) Stats() (storage.Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.flushPending(); err != nil {
		return storage.EmptyStats(), err
	}
	snapshot, err := s.store.LoadStats()
	if err != nil {
		return storage.EmptyStats(), err
	}
	return snapshot.Stats, nil
}

func (s *Service) apply(player int, action engine.Action) (engine.View, error) {
	before := s.state
	after, err := engine.Apply(before, player, action)
	if err != nil {
		return engine.View{}, err
	}
	s.updateRoundProgress(player, before, after, action)
	s.state = after
	s.settleIfNeeded()
	return s.consumeView(), nil
}

func (s *Service) consumeView() engine.View {
	view := engine.ViewFor(s.state, 0)
	s.state.Events = nil
	return view
}

func newGameID() (string, error) {
	var data [16]byte
	if _, err := cryptorand.Read(data[:]); err != nil {
		return "", fmt.Errorf("generate game id: %w", err)
	}
	return hex.EncodeToString(data[:]), nil
}
