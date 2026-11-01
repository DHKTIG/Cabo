package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Stats 跨局累积的生涯记录，用来做「递进奖励」的那层底座：
// 单轮最低点数、最长连击、零失误轮数、CABO 成功率。
//
// 和音量设置同一个理由存服务端文件：原生窗口每次随机分配端口，
// 浏览器把 127.0.0.1:61862 和 127.0.0.1:52341 当成不同来源，
// localStorage 每次启动都归零。
type Stats struct {
	BestRound  int `json:"bestRound"`  // 单轮最低点数，越小越好；-1 表示还没记录过
	BestStreak int `json:"bestStreak"` // 单轮最长连击；-1 表示还没记录过
	Rounds     int `json:"rounds"`     // 打完结算的轮数
	Perfect    int `json:"perfect"`    // 自己零失误的轮数
	Cabos      int `json:"cabos"`      // 自己喊 CABO 的次数
	CabosWon   int `json:"cabosWon"`   // 其中赌赢（得 0 分）的次数
}

// 未记录用 -1 而不是 0：0 点是这一局能拿到的最好结果，不能拿来当空值。
const noRecord = -1

// statsResponse 失败也要回 200 + ok:false：前端在原生窗口里，弹不出跨域错误页。
type statsResponse struct {
	OK    bool   `json:"ok"`
	Stats *Stats `json:"stats,omitempty"`
}

func statsPath() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "Cabo", "stats.json")
	}
	return filepath.Join(os.TempDir(), "cabo-stats.json")
}

func loadStats() Stats {
	var s Stats
	// 读不到或文件损坏都退回空记录，不让一次坏档毁掉整局游戏
	if b, err := os.ReadFile(statsPath()); err == nil && json.Unmarshal(b, &s) == nil {
		return clampStats(s)
	}
	return Stats{BestRound: noRecord, BestStreak: noRecord}
}

// clampStats 收拾一遍外部输入：计数不能为负，喊赢不可能多于喊过，
// 两个 best 只允许是 -1（没记录）或正数，中间的脏值一律折回去。
func clampStats(s Stats) Stats {
	if s.BestRound < 0 {
		s.BestRound = noRecord
	}
	if s.BestStreak < 0 {
		s.BestStreak = noRecord
	}
	if s.Rounds < 0 {
		s.Rounds = 0
	}
	if s.Perfect < 0 {
		s.Perfect = 0
	}
	if s.Cabos < 0 {
		s.Cabos = 0
	}
	if s.CabosWon > s.Cabos {
		s.CabosWon = s.Cabos
	}
	if s.CabosWon < 0 {
		s.CabosWon = 0
	}
	return s
}

// mergeStats 把一次结算并进历史。计数按增量累加，两个 best 各取更好的那个。
func mergeStats(cur, in Stats) Stats {
	out := clampStats(cur)
	out.Rounds += max(0, in.Rounds)
	out.Perfect += max(0, in.Perfect)
	out.Cabos += max(0, in.Cabos)
	out.CabosWon += max(0, in.CabosWon)

	// 连击为 0 的轮次不值得占用「有记录」这个状态，留到真的连起来过一次再写
	if in.BestStreak > 0 && in.BestStreak > out.BestStreak {
		out.BestStreak = in.BestStreak
	}
	// 0 点是可能的最好结果，所以只要带上来就当作有效样本
	if in.BestRound >= 0 && (out.BestRound == noRecord || in.BestRound < out.BestRound) {
		out.BestRound = in.BestRound
	}
	return clampStats(out)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.Method == http.MethodPost {
		var in Stats
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in); err != nil {
			writeJSON(w, statsResponse{OK: false})
			return
		}
		merged := mergeStats(loadStats(), in)
		path := statsPath()
		b, err := json.Marshal(merged)
		if os.MkdirAll(filepath.Dir(path), 0o755) != nil || err != nil || os.WriteFile(path, b, 0o644) != nil {
			writeJSON(w, statsResponse{OK: false, Stats: &merged})
			return
		}
		writeJSON(w, statsResponse{OK: true, Stats: &merged})
		return
	}

	st := loadStats()
	writeJSON(w, statsResponse{OK: true, Stats: &st})
}
