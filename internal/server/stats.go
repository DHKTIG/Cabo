package server

import (
	"net/http"

	"cabo/internal/storage"
)

type statsResponse struct {
	OK    bool           `json:"ok"`
	Stats *storage.Stats `json:"stats,omitempty"`
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	stats, err := s.game.Stats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "读取统计失败", true)
		return
	}
	writeJSON(w, statsResponse{OK: true, Stats: &stats})
}
