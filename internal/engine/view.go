package engine

// CardView 一张牌的公开信息。牌面只有数字和技能，没有花色。
type CardView struct {
	ID      int    `json:"id"`
	Label   string `json:"label"`
	Points  int    `json:"points"`
	Power   string `json:"power"`
	PowerZh string `json:"powerZh"`
}

func newCardView(c Card) CardView {
	return CardView{
		ID:      c.ID,
		Label:   c.Label(),
		Points:  c.Points(),
		Power:   c.Power().String(),
		PowerZh: c.Power().Label(),
	}
}

// SlotView 某个牌位在观察者眼中的样子。
type SlotView struct {
	Visible bool      `json:"visible"`
	FaceUp  bool      `json:"faceUp"`
	Card    *CardView `json:"card,omitempty"`
}

// PlayerView 玩家视角。
type PlayerView struct {
	Name      string     `json:"name"`
	IsAI      bool       `json:"isAI"`
	Score     int        `json:"score"`
	RoundPts  int        `json:"roundPts"`
	IsTurn    bool       `json:"isTurn"`
	IsCaller  bool       `json:"isCaller"`
	Peeked    bool       `json:"peeked"`
	Hand      []SlotView `json:"hand"`
	HandCount int        `json:"handCount"`
}

// Can 当前视角下可执行的动作。
type Can struct {
	Peek           bool `json:"peek"`
	DrawDeck       bool `json:"drawDeck"`
	TakeDiscard    bool `json:"takeDiscard"`
	DiscardPending bool `json:"discardPending"`
	PlacePending   bool `json:"placePending"`
	UsePower       bool `json:"usePower"`
	CallCabo       bool `json:"callCabo"`
}

// View 发给单个客户端的报文，绝不包含该玩家无权看到的信息。
type View struct {
	Me           int          `json:"me"`
	Players      []PlayerView `json:"players"`
	DeckCount    int          `json:"deckCount"`
	DiscardTop   *CardView    `json:"discardTop"`
	DiscardUnder []CardView   `json:"discardUnder"`
	DiscardCount int          `json:"discardCount"`
	Turn         int          `json:"turn"`
	Phase        Phase        `json:"phase"`
	Caller       int          `json:"caller"`
	Pending      *CardView    `json:"pending"`
	Round        int          `json:"round"`
	Winner       int          `json:"winner"`
	GameOver     bool         `json:"gameOver"`
	Can          Can          `json:"can"`
	Events       []Event      `json:"events"`
}

// ViewFor 裁剪出某个玩家能看到的状态。
func ViewFor(s State, me int) View {
	v := View{
		Me:           me,
		DeckCount:    len(s.Deck),
		DiscardCount: len(s.Discard),
		Turn:         s.Turn,
		Phase:        s.Phase,
		Caller:       s.Caller,
		Round:        s.Round,
		Winner:       s.Winner,
		GameOver:     s.Phase == PhaseGameOver,
		Events:       make([]Event, len(s.Events)),
	}
	copy(v.Events, s.Events)
	if len(s.Discard) > 0 {
		cv := newCardView(s.Discard[len(s.Discard)-1])
		v.DiscardTop = &cv
		// 顶牌之下的最近几张也是公开信息，前端拿去做弃牌扇
		for i := len(s.Discard) - 2; i >= 0 && len(v.DiscardUnder) < 3; i-- {
			v.DiscardUnder = append(v.DiscardUnder, newCardView(s.Discard[i]))
		}
	}
	if s.Pending != nil && s.Turn == me {
		cv := newCardView(*s.Pending)
		v.Pending = &cv
	}

	for i, p := range s.Players {
		pv := PlayerView{
			Name:      p.Name,
			IsAI:      p.IsAI,
			Score:     p.Score,
			RoundPts:  p.RoundPts,
			IsTurn:    s.Turn == i && (s.Phase == PhaseTurn || s.Phase == PhasePlace),
			IsCaller:  s.Caller == i,
			Peeked:    i < len(s.Peeked) && s.Peeked[i],
			HandCount: len(p.Hand),
		}
		for _, sl := range p.Hand {
			sv := SlotView{FaceUp: sl.FaceUp}
			if sl.FaceUp || sl.KnownByPlayer(me) {
				cv := newCardView(sl.Card)
				sv.Visible = true
				sv.Card = &cv
			}
			pv.Hand = append(pv.Hand, sv)
		}
		v.Players = append(v.Players, pv)
	}

	if me >= 0 && me < len(s.Players) && s.Turn == me {
		switch s.Phase {
		case PhasePeek:
			v.Can.Peek = !s.Peeked[me]
		case PhaseTurn:
			v.Can.DrawDeck = true
			v.Can.TakeDiscard = len(s.Discard) > 0
			v.Can.CallCabo = s.Caller < 0
		case PhasePlace:
			v.Can.DiscardPending = true
			v.Can.PlacePending = true
			v.Can.UsePower = s.Pending != nil && s.Pending.Power() != PowerNone
		}
	}
	if s.Phase == PhasePeek && me >= 0 && me < len(s.Peeked) && !s.Peeked[me] {
		v.Can.Peek = true
	}
	return v
}
