package server

import (
	"net/http"

	"cabo/internal/storage"
)

type prefsResponse struct {
	OK    bool           `json:"ok"`
	Prefs *storage.Prefs `json:"prefs,omitempty"`
}

func (s *Server) handlePrefs(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, http.MethodPost) {
		return
	}
	if r.Method == http.MethodPost {
		var prefs storage.Prefs
		if !decodeJSON(w, r, &prefs, 4096) {
			return
		}
		prefs = storage.ClampPrefs(prefs)
		if err := s.store.SavePrefs(prefs); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "保存设置失败", true)
			return
		}
		writeJSON(w, prefsResponse{OK: true, Prefs: &prefs})
		return
	}

	prefs, found, err := s.store.LoadPrefs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "读取设置失败", true)
		return
	}
	if !found {
		writeJSON(w, prefsResponse{OK: true})
		return
	}
	writeJSON(w, prefsResponse{OK: true, Prefs: &prefs})
}
