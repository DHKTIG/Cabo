package storage

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestPrefsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)
	if got, found, err := store.LoadPrefs(); err != nil || found || got != (Prefs{}) {
		t.Fatalf("missing preferences = %+v, found=%v, %v", got, found, err)
	}
	want := Prefs{Music: 0.25, Sfx: 0.75, Muted: true}
	if err := store.SavePrefs(want); err != nil {
		t.Fatal(err)
	}
	if got, found, err := store.LoadPrefs(); err != nil || !found || got != want {
		t.Fatalf("preferences = %+v, found=%v, %v; want %+v", got, found, err, want)
	}
	if err := store.SavePrefs(Prefs{Music: math.NaN(), Sfx: 4}); err != nil {
		t.Fatal(err)
	}
	if got, found, err := store.LoadPrefs(); err != nil || !found || got.Music != 0 || got.Sfx != 1 {
		t.Fatalf("clamped preferences = %+v, found=%v, %v", got, found, err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "prefs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc prefsDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Version != Version || doc.Prefs != (Prefs{Sfx: 1}) {
		t.Fatalf("unexpected v2 document: %+v", doc)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "prefs.json" {
		t.Fatalf("atomic write left temporary files: %+v", entries)
	}
}

func TestLegacyDocumentsRemainReadable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "prefs.json"), []byte(`{"music":0.4,"sfx":0.8,"muted":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyStats := Stats{BestRound: 7, BestStreak: 3, Rounds: 4, Perfect: 2, Cabos: 2, CabosWon: 1}
	data, err := json.Marshal(legacyStats)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stats.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	store := New(dir)
	prefs, found, err := store.LoadPrefs()
	if err != nil || !found || prefs != (Prefs{Music: 0.4, Sfx: 0.8, Muted: true}) {
		t.Fatalf("legacy preferences = %+v, found=%v, %v", prefs, found, err)
	}
	snapshot, err := store.LoadStats()
	if err != nil || snapshot.Stats != legacyStats {
		t.Fatalf("legacy stats = %+v, %v", snapshot.Stats, err)
	}
	if err := store.SavePrefs(prefs); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStats(snapshot); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"prefs.json", "stats.json"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		var envelope struct {
			Version int `json:"version"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Version != Version {
			t.Fatalf("%s was not migrated to v%d: %s", name, Version, data)
		}
	}
}

func TestCorruptDocumentsAreQuarantined(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"prefs.json", "stats.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{broken`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	store := New(dir)
	if prefs, found, err := store.LoadPrefs(); err != nil || found || prefs != (Prefs{}) {
		t.Fatalf("corrupt preferences = %+v, found=%v, %v", prefs, found, err)
	}
	if snapshot, err := store.LoadStats(); err != nil || snapshot.Stats != EmptyStats() {
		t.Fatalf("corrupt stats = %+v, %v", snapshot.Stats, err)
	}
	backups, err := filepath.Glob(filepath.Join(dir, "*.corrupt-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 2 {
		t.Fatalf("expected two quarantined files, got %v", backups)
	}
}

func TestStatsSnapshotAndMerge(t *testing.T) {
	store := New(t.TempDir())
	want := StatsSnapshot{
		Stats:       Stats{BestRound: 0, BestStreak: 5, Rounds: 3, Perfect: 2, Cabos: 2, CabosWon: 1},
		Settlements: map[string]struct{}{"game-1:round-1": {}},
	}
	if err := store.SaveStats(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadStats()
	if err != nil {
		t.Fatal(err)
	}
	if got.Stats != want.Stats || len(got.Settlements) != 1 {
		t.Fatalf("snapshot = %+v", got)
	}
	if _, ok := got.Settlements["game-1:round-1"]; !ok {
		t.Fatal("settlement ledger was not persisted")
	}

	merged := MergeStats(EmptyStats(), Stats{BestRound: 12, BestStreak: 2, Rounds: 1, Perfect: 1, Cabos: 1})
	if merged != (Stats{BestRound: 12, BestStreak: 2, Rounds: 1, Perfect: 1, Cabos: 1}) {
		t.Fatalf("merged stats = %+v", merged)
	}
	clamped := ClampStats(Stats{BestRound: -2, BestStreak: -3, Rounds: 2, Perfect: 9, Cabos: -1, CabosWon: 4})
	if clamped != (Stats{BestRound: NoRecord, BestStreak: NoRecord, Rounds: 2, Perfect: 2}) {
		t.Fatalf("clamped stats = %+v", clamped)
	}
	if got := ClampStats(Stats{Rounds: 1, Cabos: 1, CabosWon: -1}); got.CabosWon != 0 {
		t.Fatalf("negative wins were not clamped: %+v", got)
	}
	if got := ClampStats(Stats{BestRound: -1, BestStreak: -1, Rounds: -1, Perfect: -1, Cabos: 5, CabosWon: 9}); got != EmptyStats() {
		t.Fatalf("negative counters were not clamped: %+v", got)
	}
}

func TestConcurrentWritesRemainReadable(t *testing.T) {
	store := New(t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(rounds int) {
			defer wg.Done()
			if err := store.SaveStats(StatsSnapshot{Stats: Stats{BestRound: NoRecord, BestStreak: NoRecord, Rounds: rounds}}); err != nil {
				t.Errorf("save stats: %v", err)
			}
		}(i + 1)
	}
	wg.Wait()
	snapshot, err := store.LoadStats()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Stats.Rounds < 1 || snapshot.Stats.Rounds > 32 {
		t.Fatalf("invalid concurrent result: %+v", snapshot.Stats)
	}
}

func TestUnsupportedAndMalformedV2DocumentsAreQuarantined(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		content string
		load    func(*Store) error
	}{
		{
			name: "prefs unsupported", file: "prefs.json", content: `{"version":99,"prefs":{}}`,
			load: func(store *Store) error { _, _, err := store.LoadPrefs(); return err },
		},
		{
			name: "prefs malformed v2", file: "prefs.json", content: `{"version":2,"prefs":{"music":"bad"}}`,
			load: func(store *Store) error { _, _, err := store.LoadPrefs(); return err },
		},
		{
			name: "stats unsupported", file: "stats.json", content: `{"version":99,"stats":{}}`,
			load: func(store *Store) error { _, err := store.LoadStats(); return err },
		},
		{
			name: "stats malformed v2", file: "stats.json", content: `{"version":2,"stats":{"rounds":"bad"}}`,
			load: func(store *Store) error { _, err := store.LoadStats(); return err },
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, test.file), []byte(test.content), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := test.load(New(dir)); err != nil {
				t.Fatal(err)
			}
			backups, err := filepath.Glob(filepath.Join(dir, "*.corrupt-*"))
			if err != nil || len(backups) != 1 {
				t.Fatalf("backups = %v, %v", backups, err)
			}
		})
	}
}

func TestLoadReportsFilesystemErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "prefs.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := New(dir).LoadPrefs(); err == nil {
		t.Fatal("expected preferences read error")
	}
	if err := os.Remove(filepath.Join(dir, "prefs.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "stats.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir).LoadStats(); err == nil {
		t.Fatal("expected stats read error")
	}
}

func TestUpdateStatsTransactions(t *testing.T) {
	store := New(t.TempDir())
	updated, err := store.UpdateStats(func(snapshot StatsSnapshot) (StatsSnapshot, error) {
		snapshot.Stats.Rounds = 1
		return snapshot, nil
	})
	if err != nil || updated.Stats.Rounds != 1 {
		t.Fatalf("updated snapshot = %+v, %v", updated, err)
	}
	unchanged, err := store.UpdateStats(func(snapshot StatsSnapshot) (StatsSnapshot, error) {
		return snapshot, ErrNoChange
	})
	if err != nil || unchanged.Stats.Rounds != 1 {
		t.Fatalf("unchanged snapshot = %+v, %v", unchanged, err)
	}
	wantErr := errors.New("transaction failed")
	if _, err := store.UpdateStats(func(snapshot StatsSnapshot) (StatsSnapshot, error) {
		return snapshot, wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("transaction error = %v", err)
	}
	loaded, err := store.LoadStats()
	if err != nil || loaded.Stats.Rounds != 1 {
		t.Fatalf("failed transaction changed data: %+v, %v", loaded, err)
	}
}

func TestNewUsesUserCacheDirectory(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("LOCALAPPDATA", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
	store := New("")
	if err := store.SavePrefs(Prefs{Music: 0.5}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cache, "Cabo", "prefs.json")); err != nil {
		t.Fatalf("default store did not use the cache directory: %v", err)
	}
}

func TestNewFallsBackToTemporaryDirectory(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("TMP", temp)
	t.Setenv("TEMP", temp)
	t.Setenv("TMPDIR", temp)
	store := New("")
	if err := store.SavePrefs(Prefs{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(temp, "Cabo", "prefs.json")); err != nil {
		t.Fatalf("fallback store did not use the temporary directory: %v", err)
	}
}

func TestLegacyMalformedValuesAreQuarantined(t *testing.T) {
	for _, test := range []struct {
		name    string
		file    string
		content string
		load    func(*Store) error
	}{
		{name: "prefs", file: "prefs.json", content: `{"music":"bad"}`, load: func(store *Store) error { _, _, err := store.LoadPrefs(); return err }},
		{name: "stats", file: "stats.json", content: `{"rounds":"bad"}`, load: func(store *Store) error { _, err := store.LoadStats(); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, test.file), []byte(test.content), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := test.load(New(dir)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAtomicWriteErrorPaths(t *testing.T) {
	dir := t.TempDir()
	if err := writeJSONAtomic(filepath.Join(dir, "invalid.json"), math.Inf(1)); err == nil {
		t.Fatal("expected JSON encoding error")
	}
	longPath := filepath.Join(dir, strings.Repeat("x", 300))
	if err := writeJSONAtomic(longPath, Prefs{}); err == nil {
		t.Fatal("expected temporary file creation error")
	}
	target := filepath.Join(dir, "directory")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONAtomic(target, Prefs{}); err == nil {
		t.Fatal("expected atomic replace error")
	}
}

func TestUpdateStatsPropagatesLoadAndSaveErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "stats.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := New(dir)
	if _, err := store.UpdateStats(func(snapshot StatsSnapshot) (StatsSnapshot, error) {
		return snapshot, nil
	}); err == nil {
		t.Fatal("expected load error")
	}

	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(blocker).UpdateStats(func(snapshot StatsSnapshot) (StatsSnapshot, error) {
		return snapshot, nil
	}); err == nil {
		t.Fatal("expected save error")
	}
}

func TestSaveReportsFilesystemErrors(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := New(blocker).SavePrefs(Prefs{}); err == nil {
		t.Fatal("expected filesystem error")
	}
}
