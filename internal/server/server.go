package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"cabo/internal/ai"
	"cabo/internal/engine"
)

//go:embed static/*
var staticFS embed.FS

// Server 单机版只需要维护一局游戏。
type Server struct {
	mu    sync.Mutex
	state engine.State
	rng   *rand.Rand
	ready bool
	level ai.Level
	mux   *http.ServeMux
}

// New 构造 HTTP 服务。
func New() *Server {
	s := &Server{}
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/new", s.handleNew)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/action", s.handleAction)
	mux.HandleFunc("/api/ai", s.handleAI)
	mux.HandleFunc("/api/next", s.handleNext)
	mux.HandleFunc("/api/prefs", s.handlePrefs)
	s.mux = mux
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

type response struct {
	OK    bool         `json:"ok"`
	Ready bool         `json:"ready"`
	Error string       `json:"error,omitempty"`
	View  *engine.View `json:"view,omitempty"`
}

func clampLevel(v int) int {
	if v < 0 {
		return 0
	}
	if v > 2 {
		return 2
	}
	return v
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) reply(w http.ResponseWriter, err error) {
	res := response{OK: err == nil, Ready: s.ready}
	if err != nil {
		res.Error = err.Error()
	} else {
		v := engine.ViewFor(s.state, 0)
		res.View = &v
		// 事件只投递一次，避免客户端重复播放动画
		s.state.Events = nil
	}
	writeJSON(w, res)
}

func (s *Server) handleNew(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req struct {
		Name  string `json:"name"`
		AI    int    `json:"ai"`
		Level int    `json:"level"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.AI < 1 {
		req.AI = 1
	}
	if req.AI > 3 {
		req.AI = 3
	}
	s.level = ai.Level(clampLevel(req.Level))
	name := req.Name
	if name == "" {
		name = "你"
	}

	names := []string{name}
	flags := []bool{false}
	letters := []string{"电脑 A", "电脑 B", "电脑 C"}
	for i := 0; i < req.AI; i++ {
		names = append(names, letters[i])
		flags = append(flags, true)
	}

	s.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	s.state = engine.NewGame(names, flags, s.rng)
	s.ready = true

	// AI 立即完成开局看牌
	for i := 1; i < len(names); i++ {
		if st, err := engine.Apply(s.state, i, ai.Decide(s.state, i, s.rng, s.level)); err == nil {
			s.state = st
		}
	}
	s.reply(w, nil)
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready {
		writeJSON(w, response{OK: true, Ready: false})
		return
	}
	s.reply(w, nil)
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready {
		s.reply(w, errNotReady)
		return
	}
	var a engine.Action
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		s.reply(w, errBadRequest)
		return
	}
	st, err := engine.Apply(s.state, 0, a)
	if err != nil {
		s.reply(w, err)
		return
	}
	s.state = st
	s.reply(w, nil)
}

func (s *Server) handleAI(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready {
		s.reply(w, errNotReady)
		return
	}

	// 开局阶段：找出还没看牌的 AI
	if s.state.Phase == engine.PhasePeek {
		for i := 1; i < len(s.state.Players); i++ {
			if !s.state.Peeked[i] {
				st, err := engine.Apply(s.state, i, ai.Decide(s.state, i, s.rng, s.level))
				if err != nil {
					s.reply(w, err)
					return
				}
				s.state = st
				s.reply(w, nil)
				return
			}
		}
		s.reply(w, nil)
		return
	}

	if s.state.Phase != engine.PhaseTurn && s.state.Phase != engine.PhasePlace {
		s.reply(w, nil)
		return
	}
	t := s.state.Turn
	if t <= 0 || t >= len(s.state.Players) {
		s.reply(w, nil)
		return
	}
	st, err := engine.Apply(s.state, t, ai.Decide(s.state, t, s.rng, s.level))
	if err != nil {
		s.reply(w, err)
		return
	}
	s.state = st
	s.reply(w, nil)
}

func (s *Server) handleNext(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready {
		s.reply(w, errNotReady)
		return
	}
	s.state = engine.NextRound(s.state, s.rng)
	for i := 1; i < len(s.state.Players); i++ {
		if st, err := engine.Apply(s.state, i, ai.Decide(s.state, i, s.rng, s.level)); err == nil {
			s.state = st
		}
	}
	s.reply(w, nil)
}
