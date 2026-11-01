package engine

// EventKind 对局事件的类型。前端靠这些事件驱动动画，而不是靠文字日志。
type EventKind string

const (
	EvRoundStart EventKind = "roundStart"
	EvPeek       EventKind = "peek"
	EvDraw       EventKind = "draw"
	EvReshuffle  EventKind = "reshuffle"
	EvDiscard    EventKind = "discard"
	EvPlace      EventKind = "place"
	EvTake       EventKind = "take"
	EvPenalty    EventKind = "penalty"
	EvSpy        EventKind = "spy"
	EvSwap       EventKind = "swap"
	EvCabo       EventKind = "cabo"
	EvScore      EventKind = "score"
	EvReset      EventKind = "reset"
	EvGameOver   EventKind = "gameOver"
)

// Event 一次可观察的变化。
// Card 用 CardView 而不是内部 Card：前端要直接拿 label/power 渲染，内部类型没有这些字段。
type Event struct {
	Kind    EventKind `json:"kind"`
	Player  int       `json:"player"`
	Card    *CardView `json:"card,omitempty"`
	Target  *Target   `json:"target,omitempty"`
	Target2 *Target   `json:"target2,omitempty"`
	Value   int       `json:"value,omitempty"`
	Text    string    `json:"text,omitempty"`
}

// cardViewPtr 把内部牌转成可展示的视图。
func cardViewPtr(c Card) *CardView {
	v := newCardView(c)
	return &v
}

func (s *State) emit(e Event) {
	s.Events = append(s.Events, e)
	if len(s.Events) > 200 {
		s.Events = s.Events[len(s.Events)-200:]
	}
}
