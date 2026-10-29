package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"cabo/internal/engine"
)

func call(t *testing.T, base, path string, body any) response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest("POST", base+path, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("%s 返回状态码 %d", path, resp.StatusCode)
	}
	var r response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestStaticAssetsEmbedded(t *testing.T) {
	srv := httptest.NewServer(New())
	defer srv.Close()

	for _, p := range []string{"/", "/style.css", "/app.js"} {
		resp, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("%s 应可访问，实际 %d", p, resp.StatusCode)
		}
		if resp.ContentLength == 0 {
			t.Fatalf("%s 内容为空", p)
		}
	}
}

// TestFullGameOverHTTP 通过真实 HTTP 接口打完整局，验证没有非法状态、没有死循环、牌数守恒。
func TestFullGameOverHTTP(t *testing.T) {
	srv := httptest.NewServer(New())
	defer srv.Close()

	rng := rand.New(rand.NewSource(2026))

	r := call(t, srv.URL, "/api/new", map[string]any{"name": "测试员", "ai": 3})
	if !r.OK || r.View == nil {
		t.Fatalf("开局失败：%v", r.Error)
	}
	v := r.View

	r = call(t, srv.URL, "/api/action", map[string]any{"type": "peek", "indices": []int{0, 1}})
	if !r.OK {
		t.Fatalf("看牌失败：%v", r.Error)
	}
	v = r.View

	ctx := "开局"
	checkTotal := func(v *engine.View) {
		n := v.DeckCount + v.DiscardCount
		hands := ""
		for _, p := range v.Players {
			n += p.HandCount
			hands += fmt.Sprintf(" %s=%d", p.Name, p.HandCount)
		}
		if v.Phase == "place" {
			n++ // 手里还捏着一张待安置的牌
		}
		if n != 54 {
			t.Fatalf("牌数不守恒：%d（%s，阶段 %s，牌堆 %d，弃牌堆 %d，手牌%s）",
				n, ctx, v.Phase, v.DeckCount, v.DiscardCount, hands)
		}
	}
	checkTotal(v)

	for step := 0; step < 4000; step++ {
		if v.GameOver {
			break
		}
		if v.Phase == "roundEnd" {
			r = call(t, srv.URL, "/api/next", nil)
			if !r.OK {
				t.Fatalf("进入下一轮失败：%v", r.Error)
			}
			v = r.View
			continue
		}

		if v.Phase == "peek" {
			r = call(t, srv.URL, "/api/action", map[string]any{"type": "peek", "indices": []int{0, 1}})
			if !r.OK {
				t.Fatalf("新一轮看牌失败：%v", r.Error)
			}
			v = r.View
			ctx = fmt.Sprintf("step %d 新一轮看牌后", step)
			checkTotal(v)
			continue
		}

		if v.Turn != 0 {
			r = call(t, srv.URL, "/api/ai", nil)
			if !r.OK {
				t.Fatalf("AI 行动失败：%v", r.Error)
			}
			v = r.View
			ctx = fmt.Sprintf("step %d AI 行动后", step)
			checkTotal(v)
			continue
		}

		var body map[string]any
		if v.Can.DiscardPending {
			known := []struct {
				i, pts int
			}{}
			for i, s := range v.Players[0].Hand {
				if s.Visible {
					known = append(known, struct{ i, pts int }{i, s.Card.Points})
				}
			}
			best, bestPts := -1, -1
			for _, k := range known {
				if k.pts > bestPts {
					bestPts, best = k.pts, k.i
				}
			}
			if best >= 0 && v.Pending != nil && v.Pending.Points < bestPts-1 {
				body = map[string]any{"type": "place", "indices": []int{best}}
			} else {
				body = map[string]any{"type": "discardPending"}
			}
		} else {
			switch x := rng.Float64(); {
			case x < 0.55 || !v.Can.TakeDiscard:
				body = map[string]any{"type": "drawDeck"}
			case x < 0.92:
				body = map[string]any{
					"type":    "takeDiscard",
					"indices": []int{rng.Intn(len(v.Players[0].Hand))},
				}
			case v.Can.CallCabo:
				body = map[string]any{"type": "callCabo"}
			default:
				body = map[string]any{"type": "drawDeck"}
			}
		}

		r = call(t, srv.URL, "/api/action", body)
		if !r.OK {
			t.Fatalf("第 %d 步动作 %v 失败：%v", step, body["type"], r.Error)
		}
		v = r.View
		ctx = fmt.Sprintf("step %d 人类动作 %v", step, body["type"])
		checkTotal(v)
	}

	if !v.GameOver {
		t.Fatalf("4000 步内游戏没有结束，阶段停在 %s", v.Phase)
	}
	maxScore := 0
	for _, p := range v.Players {
		if p.Score < 0 {
			t.Fatalf("%s 出现负分 %d", p.Name, p.Score)
		}
		if p.Score > maxScore {
			maxScore = p.Score
		}
	}
	if maxScore <= 100 {
		t.Fatalf("结束时应至少有一人超过 100 分，最高只有 %d", maxScore)
	}
	if v.Winner < 0 || v.Winner >= len(v.Players) {
		t.Fatalf("获胜者下标非法：%d", v.Winner)
	}
	lowest := 0
	for i := range v.Players {
		if v.Players[i].Score < v.Players[lowest].Score {
			lowest = i
		}
	}
	if v.Winner != lowest {
		t.Fatalf("获胜者应是分数最低的人")
	}
}

func TestActionBeforeStart(t *testing.T) {
	srv := httptest.NewServer(New())
	defer srv.Close()
	r := call(t, srv.URL, "/api/action", map[string]any{"type": "drawDeck"})
	if r.OK {
		t.Fatal("没开局就行动应被拒绝")
	}
}
