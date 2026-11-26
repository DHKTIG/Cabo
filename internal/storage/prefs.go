package storage

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type Prefs struct {
	Music float64 `json:"music"`
	Sfx   float64 `json:"sfx"`
	Muted bool    `json:"muted"`
}

type prefsDocument struct {
	Version int   `json:"version"`
	Prefs   Prefs `json:"prefs"`
}

func ClampPrefs(prefs Prefs) Prefs {
	prefs.Music = clampUnit(prefs.Music)
	prefs.Sfx = clampUnit(prefs.Sfx)
	return prefs
}

func clampUnit(value float64) float64 {
	if math.IsNaN(value) || value <= 0 {
		return 0
	}
	if value >= 1 {
		return 1
	}
	return value
}

func (s *Store) LoadPrefs() (Prefs, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.prefsPath)
	if os.IsNotExist(err) {
		return Prefs{}, false, nil
	}
	if err != nil {
		return Prefs{}, false, fmt.Errorf("read preferences: %w", err)
	}
	var envelope struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		if err := quarantine(s.prefsPath); err != nil {
			return Prefs{}, false, err
		}
		return Prefs{}, false, nil
	}
	if envelope.Version == Version {
		var doc prefsDocument
		if err := json.Unmarshal(data, &doc); err != nil {
			if err := quarantine(s.prefsPath); err != nil {
				return Prefs{}, false, err
			}
			return Prefs{}, false, nil
		}
		return ClampPrefs(doc.Prefs), true, nil
	}
	if envelope.Version != 0 {
		if err := quarantine(s.prefsPath); err != nil {
			return Prefs{}, false, err
		}
		return Prefs{}, false, nil
	}
	var legacy Prefs
	if err := json.Unmarshal(data, &legacy); err != nil {
		if err := quarantine(s.prefsPath); err != nil {
			return Prefs{}, false, err
		}
		return Prefs{}, false, nil
	}
	return ClampPrefs(legacy), true, nil
}

func (s *Store) SavePrefs(prefs Prefs) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeJSONAtomic(s.prefsPath, prefsDocument{Version: Version, Prefs: ClampPrefs(prefs)})
}
