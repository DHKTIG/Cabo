package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const Version = 2

var ErrNoChange = errors.New("storage transaction produced no change")

type Store struct {
	mu        sync.Mutex
	prefsPath string
	statsPath string
}

func New(dir string) *Store {
	if dir == "" {
		if cache, err := os.UserCacheDir(); err == nil {
			dir = filepath.Join(cache, "Cabo")
		} else {
			dir = filepath.Join(os.TempDir(), "Cabo")
		}
	}
	return &Store{
		prefsPath: filepath.Join(dir, "prefs.json"),
		statsPath: filepath.Join(dir, "stats.json"),
	}
}

func quarantine(path string) error {
	backup := fmt.Sprintf("%s.corrupt-%d", path, time.Now().UnixNano())
	if err := os.Rename(path, backup); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("quarantine %s: %w", path, err)
	}
	return nil
}

func writeJSONAtomic(path string, value any) (err error) {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempPath := file.Name()
	defer func() {
		_ = file.Close()
		_ = os.Remove(tempPath)
	}()
	if err := file.Chmod(0o644); err != nil {
		return fmt.Errorf("set data permissions: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write data: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync data: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace data: %w", err)
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}
