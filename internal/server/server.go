package server

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"cabo/internal/engine"
	"cabo/internal/game"
	"cabo/internal/storage"
)

//go:embed static/*
var staticFS embed.FS

type Server struct {
	store *storage.Store
	game  *game.Service
	mux   *http.ServeMux
	token string
}

func New() *Server {
	store := storage.New("")
	s := &Server{
		store: store,
		game:  game.New(store, time.Now().UnixNano()),
		token: os.Getenv("CABO_API_TOKEN"),
	}
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", staticHandler(sub))
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/new", s.handleNew)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/action", s.handleAction)
	mux.HandleFunc("/api/ai", s.handleAI)
	mux.HandleFunc("/api/next", s.handleNext)
	mux.HandleFunc("/api/prefs", s.handlePrefs)
	mux.HandleFunc("/api/stats", s.handleStats)
	s.mux = mux
	return s
}

func staticHandler(fsys fs.FS) http.Handler {
	files := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch path.Ext(r.URL.Path) {
		case ".html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case ".css":
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case ".js", ".mjs":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		}
		files.ServeHTTP(w, r)
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if !validHost(r.Host) {
		writeError(w, http.StatusForbidden, "forbidden", "Host 不允许", false)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/healthz" {
		if !validOrigin(r) {
			writeError(w, http.StatusForbidden, "forbidden", "Origin 不允许", false)
			return
		}
		if !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "访问令牌无效", false)
			return
		}
	}
	s.mux.ServeHTTP(w, r)
}

type response struct {
	OK        bool         `json:"ok"`
	Ready     bool         `json:"ready"`
	Error     *apiError    `json:"error,omitempty"`
	View      *engine.View `json:"view,omitempty"`
	RequestID string       `json:"requestId"`
}

func writeJSON(w http.ResponseWriter, value any) {
	writeJSONStatus(w, http.StatusOK, value)
}

func (s *Server) writeGameResult(w http.ResponseWriter, view engine.View, err error) {
	result := response{
		OK:        err == nil,
		Ready:     !errors.Is(err, game.ErrNotReady),
		RequestID: newRequestID(),
	}
	if err != nil {
		status, code, message := parseActionError(err)
		result.Error = &apiError{Code: code, Message: message, Retryable: status >= 500}
		writeJSONStatus(w, status, result)
		return
	}
	result.View = &view
	writeJSON(w, result)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleNew(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var request struct {
		Name  string `json:"name"`
		AI    int    `json:"ai"`
		Level int    `json:"level"`
	}
	if !decodeJSON(w, r, &request, 4096) {
		return
	}
	view, err := s.game.NewGame(request.Name, request.AI, request.Level)
	s.writeGameResult(w, view, err)
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	view, ready := s.game.Snapshot()
	if !ready {
		writeJSON(w, response{OK: true, Ready: false, RequestID: newRequestID()})
		return
	}
	writeJSON(w, response{OK: true, Ready: true, View: &view, RequestID: newRequestID()})
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var action engine.Action
	if !decodeJSON(w, r, &action, 16*1024) {
		return
	}
	view, err := s.game.Apply(action)
	s.writeGameResult(w, view, err)
}

func (s *Server) handleAI(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct{}
	if r.ContentLength != 0 && !decodeJSON(w, r, &body, 4096) {
		return
	}
	view, err := s.game.AdvanceAI()
	s.writeGameResult(w, view, err)
}

func (s *Server) handleNext(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct{}
	if r.ContentLength != 0 && !decodeJSON(w, r, &body, 4096) {
		return
	}
	view, err := s.game.NextRound()
	s.writeGameResult(w, view, err)
}
