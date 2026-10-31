package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// prefsDoc 是音量与静音设置。
//
// 存在服务端文件而不是 localStorage：原生窗口默认随机分配端口，
// 浏览器会把 127.0.0.1:61862 和 127.0.0.1:52341 当成两个不同来源，
// 于是每次启动设置都归零。
type prefsDoc struct {
	Music float64 `json:"music"`
	Sfx   float64 `json:"sfx"`
	Muted bool    `json:"muted"`
}

// prefsPath 日志目录的第一选择：用户缓存目录。
func prefsPath() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "Cabo", "prefs.json")
	}
	return filepath.Join(os.TempDir(), "cabo-prefs.json")
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func (s *Server) handlePrefs(w http.ResponseWriter, r *http.Request) {
	type prefsResponse struct {
		OK    bool      `json:"ok"`
		Prefs *prefsDoc `json:"prefs,omitempty"`
	}

	if r.Method == http.MethodPost {
		var p prefsDoc
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&p); err != nil {
			writeJSON(w, prefsResponse{OK: false})
			return
		}
		p.Music, p.Sfx = clampUnit(p.Music), clampUnit(p.Sfx)
		path := prefsPath()
		if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
			writeJSON(w, prefsResponse{OK: false})
			return
		}
		b, err := json.Marshal(p)
		if err != nil || os.WriteFile(path, b, 0o644) != nil {
			writeJSON(w, prefsResponse{OK: false})
			return
		}
		writeJSON(w, prefsResponse{OK: true, Prefs: &p})
		return
	}

	// 读不到或文件损坏都当作「没设置过」，前端退回默认值
	var p prefsDoc
	if b, err := os.ReadFile(prefsPath()); err == nil && json.Unmarshal(b, &p) == nil {
		p.Music, p.Sfx = clampUnit(p.Music), clampUnit(p.Sfx)
		writeJSON(w, prefsResponse{OK: true, Prefs: &p})
		return
	}
	writeJSON(w, prefsResponse{OK: true})
}
