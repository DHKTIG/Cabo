package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMergeStats(t *testing.T) {
	cases := []struct {
		name string
		cur  Stats
		in   Stats
		want Stats
	}{
		{
			name: "首次写入",
			cur:  Stats{BestRound: noRecord, BestStreak: noRecord},
			in:   Stats{BestRound: 12, BestStreak: 3, Rounds: 1, Cabos: 1, CabosWon: 1, Perfect: 1},
			want: Stats{BestRound: 12, BestStreak: 3, Rounds: 1, Perfect: 1, Cabos: 1, CabosWon: 1},
		},
		{
			name: "单轮点数只认更低的",
			cur:  Stats{BestRound: 8, BestStreak: 4},
			in:   Stats{BestRound: 11, BestStreak: 2, Rounds: 1},
			want: Stats{BestRound: 8, BestStreak: 4, Rounds: 1},
		},
		{
			name: "0 点是有效记录，不是空值",
			cur:  Stats{BestRound: 6, BestStreak: 2, Rounds: 3},
			in:   Stats{BestRound: 0, Rounds: 1, Cabos: 1},
			want: Stats{BestRound: 0, BestStreak: 2, Rounds: 4, Cabos: 1, CabosWon: 0},
		},
		{
			name: "连击为 0 不占用「有记录」",
			cur:  Stats{BestRound: 9, BestStreak: noRecord},
			in:   Stats{BestRound: 9, BestStreak: 0, Rounds: 1},
			want: Stats{BestRound: 9, BestStreak: noRecord, Rounds: 1},
		},
		{
			name: "脏输入折回去",
			cur:  Stats{BestRound: -7, BestStreak: -3, Rounds: -5, Cabos: 1, CabosWon: 4},
			in:   Stats{BestRound: -1, Cabos: -2},
			want: Stats{BestRound: noRecord, BestStreak: noRecord, Rounds: 0, Cabos: 1, CabosWon: 1},
		},
	}

	for _, c := range cases {
		got := mergeStats(c.cur, c.in)
		if got != c.want {
			t.Errorf("%s：期望 %+v，实际 %+v", c.name, c.want, got)
		}
	}
}

// TestStatsRoundTrip 记录要跨启动保留：合并后再读回应是同一组值。
func TestStatsRoundTrip(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv := httptest.NewServer(New())
	defer srv.Close()

	get := func() Stats {
		t.Helper()
		resp, err := http.Get(srv.URL + "/api/stats")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var r struct {
			OK    bool   `json:"ok"`
			Stats *Stats `json:"stats"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			t.Fatal(err)
		}
		if !r.OK || r.Stats == nil {
			t.Fatalf("读取失败：%+v", r)
		}
		return *r.Stats
	}
	post := func(body Stats) Stats {
		t.Helper()
		b, _ := json.Marshal(body)
		resp, err := http.Post(srv.URL+"/api/stats", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var r struct {
			OK    bool   `json:"ok"`
			Stats *Stats `json:"stats"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			t.Fatal(err)
		}
		if !r.OK || r.Stats == nil {
			t.Fatalf("写入失败：%+v", r)
		}
		return *r.Stats
	}

	if got := get(); got.BestRound != noRecord || got.BestStreak != noRecord || got.Rounds != 0 {
		t.Fatalf("首次读取应是空记录，实际 %+v", got)
	}

	post(Stats{BestRound: 14, BestStreak: 3, Rounds: 1, Perfect: 1, Cabos: 2, CabosWon: 1})
	post(Stats{BestRound: 5, BestStreak: 6, Rounds: 1, Cabos: 1, CabosWon: 1})

	want := Stats{BestRound: 5, BestStreak: 6, Rounds: 2, Perfect: 1, Cabos: 3, CabosWon: 2}
	if got := get(); got != want {
		t.Fatalf("合并后 %+v，期望 %+v", got, want)
	}

	// 越界/畸形请求不能把文件写坏
	bad := httptest.NewRecorder()
	srv2 := New()
	req, _ := http.NewRequest("POST", "/api/stats", bytes.NewReader([]byte("{不是 JSON")))
	srv2.ServeHTTP(bad, req)
	var r statsResponse
	if err := json.Unmarshal(bad.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.OK {
		t.Fatal("畸形请求应被拒绝")
	}
}
