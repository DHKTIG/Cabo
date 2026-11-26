package game

import (
	"errors"
	"strconv"

	"cabo/internal/engine"
	"cabo/internal/storage"
)

type roundResult struct {
	id         string
	bestRound  int
	bestStreak int
	perfect    int
	cabos      int
	cabosWon   int
}

func (s *Service) resetRoundProgress() {
	s.streak = 0
	s.roundBest = 0
	s.mistakes = 0
	s.cabos = 0
}

func (s *Service) updateRoundProgress(player int, before, after engine.State, action engine.Action) {
	if player != 0 {
		return
	}
	if action.Type == engine.ActCallCabo && after.Caller == 0 {
		s.cabos++
	}
	penalized := false
	for _, event := range after.Events {
		if event.Kind == engine.EvPenalty && event.Player == 0 {
			s.mistakes++
			s.setStreak(0)
			penalized = true
		}
	}
	if penalized {
		return
	}

	if action.Type == engine.ActPlace && before.Pending != nil && len(action.Indices) == 1 {
		index := action.Indices[0]
		if index < 0 || index >= len(before.Players[0].Hand) {
			return
		}
		old := before.Players[0].Hand[index]
		if !old.FaceUp && !old.KnownByPlayer(0) {
			return
		}
		if before.Pending.Points() < old.Card.Points() {
			s.setStreak(s.streak + 1)
		} else {
			s.setStreak(0)
		}
		return
	}

	if action.Type == engine.ActTakeDiscard && len(before.Discard) > 0 {
		better := true
		for _, index := range action.Indices {
			if index < 0 || index >= len(before.Players[0].Hand) {
				return
			}
			old := before.Players[0].Hand[index]
			if !old.FaceUp && !old.KnownByPlayer(0) {
				return
			}
			if before.Discard[len(before.Discard)-1].Points() >= old.Card.Points() {
				better = false
			}
		}
		if better {
			s.setStreak(s.streak + 1)
		} else {
			s.setStreak(0)
		}
	}
}

func (s *Service) setStreak(value int) {
	if value < 0 {
		value = 0
	}
	if value > 99 {
		value = 99
	}
	s.streak = value
	if value > s.roundBest {
		s.roundBest = value
	}
}

func (s *Service) settleIfNeeded() {
	if s.state.Phase != engine.PhaseRoundEnd && s.state.Phase != engine.PhaseGameOver {
		return
	}
	if s.pending == nil {
		perfect := 1
		if s.mistakes > 0 {
			perfect = 0
		}
		won := 0
		if s.cabos > 0 && s.state.Caller == 0 && s.state.Players[0].RoundPts == 0 {
			won = 1
		}
		s.pending = &roundResult{
			id:         s.gameID + ":" + strconv.Itoa(s.state.Round),
			bestRound:  s.state.Players[0].RoundPts,
			bestStreak: s.roundBest,
			perfect:    perfect,
			cabos:      s.cabos,
			cabosWon:   won,
		}
	}
	_ = s.flushPending()
}

func (s *Service) flushPending() error {
	if s.pending == nil {
		return nil
	}
	result := s.pending
	_, err := s.store.UpdateStats(func(snapshot storage.StatsSnapshot) (storage.StatsSnapshot, error) {
		if snapshot.Settlements == nil {
			snapshot.Settlements = make(map[string]struct{})
		}
		if _, exists := snapshot.Settlements[result.id]; exists {
			return snapshot, storage.ErrNoChange
		}
		snapshot.Stats = storage.MergeStats(snapshot.Stats, storage.Stats{
			BestRound:  result.bestRound,
			BestStreak: result.bestStreak,
			Rounds:     1,
			Perfect:    result.perfect,
			Cabos:      result.cabos,
			CabosWon:   result.cabosWon,
		})
		snapshot.Settlements[result.id] = struct{}{}
		return snapshot, nil
	})
	if err != nil && !errors.Is(err, storage.ErrNoChange) {
		return err
	}
	s.pending = nil
	return nil
}
