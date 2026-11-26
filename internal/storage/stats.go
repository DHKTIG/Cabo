package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const NoRecord = -1

type Stats struct {
	BestRound  int `json:"bestRound"`
	BestStreak int `json:"bestStreak"`
	Rounds     int `json:"rounds"`
	Perfect    int `json:"perfect"`
	Cabos      int `json:"cabos"`
	CabosWon   int `json:"cabosWon"`
}

type StatsSnapshot struct {
	Stats       Stats
	Settlements map[string]struct{}
}

type statsDocument struct {
	Version     int                 `json:"version"`
	Stats       Stats               `json:"stats"`
	Settlements map[string]struct{} `json:"settlements,omitempty"`
}

func EmptyStats() Stats {
	return Stats{BestRound: NoRecord, BestStreak: NoRecord}
}

func ClampStats(stats Stats) Stats {
	if stats.BestRound < 0 {
		stats.BestRound = NoRecord
	}
	if stats.BestStreak < 0 {
		stats.BestStreak = NoRecord
	}
	if stats.Rounds < 0 {
		stats.Rounds = 0
	}
	if stats.Perfect < 0 {
		stats.Perfect = 0
	} else if stats.Perfect > stats.Rounds {
		stats.Perfect = stats.Rounds
	}
	if stats.Cabos < 0 {
		stats.Cabos = 0
	} else if stats.Cabos > stats.Rounds {
		stats.Cabos = stats.Rounds
	}
	if stats.CabosWon < 0 {
		stats.CabosWon = 0
	} else if stats.CabosWon > stats.Cabos {
		stats.CabosWon = stats.Cabos
	}
	return stats
}

func MergeStats(current, delta Stats) Stats {
	merged := ClampStats(current)
	delta = ClampStats(delta)
	merged.Rounds += max(0, delta.Rounds)
	merged.Perfect += max(0, delta.Perfect)
	merged.Cabos += max(0, delta.Cabos)
	merged.CabosWon += max(0, delta.CabosWon)
	if delta.BestStreak > 0 && delta.BestStreak > merged.BestStreak {
		merged.BestStreak = delta.BestStreak
	}
	if delta.BestRound >= 0 && (merged.BestRound == NoRecord || delta.BestRound < merged.BestRound) {
		merged.BestRound = delta.BestRound
	}
	return ClampStats(merged)
}

func (s *Store) LoadStats() (StatsSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadStats()
}

func (s *Store) loadStats() (StatsSnapshot, error) {
	empty := StatsSnapshot{Stats: EmptyStats()}
	data, err := os.ReadFile(s.statsPath)
	if os.IsNotExist(err) {
		return empty, nil
	}
	if err != nil {
		return empty, fmt.Errorf("read stats: %w", err)
	}
	var envelope struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		if err := quarantine(s.statsPath); err != nil {
			return empty, err
		}
		return empty, nil
	}
	if envelope.Version == Version {
		var doc statsDocument
		if err := json.Unmarshal(data, &doc); err != nil {
			if err := quarantine(s.statsPath); err != nil {
				return empty, err
			}
			return empty, nil
		}
		return StatsSnapshot{Stats: ClampStats(doc.Stats), Settlements: doc.Settlements}, nil
	}
	if envelope.Version != 0 {
		if err := quarantine(s.statsPath); err != nil {
			return empty, err
		}
		return empty, nil
	}
	var legacy Stats
	if err := json.Unmarshal(data, &legacy); err != nil {
		if err := quarantine(s.statsPath); err != nil {
			return empty, err
		}
		return empty, nil
	}
	return StatsSnapshot{Stats: ClampStats(legacy)}, nil
}

func (s *Store) SaveStats(snapshot StatsSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveStats(snapshot)
}

func (s *Store) saveStats(snapshot StatsSnapshot) error {
	doc := statsDocument{Version: Version, Stats: ClampStats(snapshot.Stats)}
	if len(snapshot.Settlements) > 0 {
		doc.Settlements = make(map[string]struct{}, len(snapshot.Settlements))
		for id := range snapshot.Settlements {
			doc.Settlements[id] = struct{}{}
		}
	}
	return writeJSONAtomic(s.statsPath, doc)
}

func (s *Store) UpdateStats(update func(StatsSnapshot) (StatsSnapshot, error)) (StatsSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, err := s.loadStats()
	if err != nil {
		return snapshot, err
	}
	updated, err := update(snapshot)
	if err != nil {
		if errors.Is(err, ErrNoChange) {
			return snapshot, nil
		}
		return snapshot, err
	}
	if err := s.saveStats(updated); err != nil {
		return snapshot, err
	}
	return updated, nil
}
