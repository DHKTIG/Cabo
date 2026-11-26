package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"cabo/internal/storage"
)

func TestStatsEndpointIgnoresClientDeltas(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv := httptest.NewServer(New())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/stats", "application/json", bytes.NewBufferString(`{"rounds":999,"bestRound":0,"bestStreak":99,"cabos":999,"cabosWon":999}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("统计写入应返回 405，实际 %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = http.Get(srv.URL + "/api/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result statsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Stats == nil || *result.Stats != storage.EmptyStats() {
		t.Fatalf("客户端增量不应写入统计：%+v", result)
	}
}

func TestConcurrentStatsRequests(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv := httptest.NewServer(New())
	defer srv.Close()

	const requests = 20
	var wg sync.WaitGroup
	errors := make(chan error, requests)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get(srv.URL + "/api/stats")
			if err != nil {
				errors <- err
				return
			}
			defer resp.Body.Close()
			var result statsResponse
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				errors <- err
				return
			}
			if !result.OK || result.Stats == nil || result.Stats.Rounds != 0 {
				errors <- fmt.Errorf("invalid stats response: %+v", result)
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}
