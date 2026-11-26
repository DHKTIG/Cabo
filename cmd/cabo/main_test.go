package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestShutdownStopsHTTPServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	shutdown(server.Config)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(server.URL)
		if err != nil {
			return
		}
		response.Body.Close()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("HTTP server did not shut down")
}
