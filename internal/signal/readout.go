package signal

import (
	"time"

	"github.com/Torwalt/turncost/internal/pricing"
	"github.com/Torwalt/turncost/internal/session"
)

// Placeholders for the reset estimate, to be fitted by the backtest.
const (
	// HandoverRequests is how many cache-read passes over the context writing
	// a handover note takes (cold: the first one is a cache write).
	HandoverRequests = 3
	// HandoverOutputTokens is the length of the handover note.
	HandoverOutputTokens = 5000
	// FreshRebuild is how many baselines of cache write a fresh session pays
	// to rebuild what the old one knew.
	FreshRebuild = 2
)

// Lookup finds the price of a model; pricing.Lookup is the table the program
// uses.
type Lookup func(model string) (pricing.Price, bool)

// Readout is what the live surfaces show about a session: its context, what
// the last turn cost and how much of that was history, what follow-ups on
// background events cost since, and what a reset would cost. All figures are
// dollars or tokens; the zero value of a Has flag means the figure is absent.
//
// A turn is the user's message up to the agent's reply: a prompt, or a command
// that ran, with the continuations after it. Requests that a background event
// (a finished task or monitor) set off after the reply are background, not
// part of the turn.
type Readout struct {
	// HasMain is false when the session has no main-thread request yet; every
	// other field is then zero.
	HasMain bool
	// Baseline (B) is the prompt of the first main request, Context (C) that
	// of the last.
	Baseline int64
	Context  int64
	// Priced is false when the last main request's model has no price; no
	// dollar figure is then set.
	Priced bool

	TTL         time.Duration
	LastRequest time.Time
	Idle        time.Duration
	Cold        bool

	// LastTurn is what the last turn with a request cost, subagents included;
	// LastTurnHistory the part of it its main requests paid to re-read context
	// beyond the baseline, at cache-read price.
	HasLastTurn     bool
	LastTurn        float64
	LastTurnHistory float64
	// Background is what requests set off by background events cost since the
	// last turn.
	Background float64

	ColdNext   float64
	Handover   float64
	FreshStart float64
	Fresh      float64
	Reset      float64

	// PaybackTurns is how many turns like the last one a reset pays back in:
	// the reset over the last turn's history. HasPayback is false when the
	// last turn carried no history.
	HasPayback   bool
	PaybackTurns float64
}

// Read computes the readout of a session at time now. It is pure: same
// session, time and prices, same figures.
func Read(s session.Session, now time.Time, lookup Lookup) Readout {
	var first, last *session.Request
	oneHour := false
	for i := range s.Turns {
		for j := range s.Turns[i].Requests {
			r := &s.Turns[i].Requests[j]
			if r.Sidechain {
				continue
			}
			if first == nil {
				first = r
			}
			last = r
			if r.Usage.CacheWrite1h > 0 {
				oneHour = true
			}
		}
	}
	if last == nil {
		return Readout{}
	}

	ro := Readout{
		HasMain:     true,
		Baseline:    first.Usage.Prompt(),
		Context:     last.Usage.Prompt(),
		TTL:         5 * time.Minute,
		LastRequest: last.Time,
		Idle:        now.Sub(last.Time),
	}
	if oneHour {
		ro.TTL = time.Hour
	}
	ro.Cold = ro.Idle > ro.TTL

	p, ok := lookup(last.Model)
	if !ok {
		return ro
	}
	ro.Priced = true

	turn, background := lastTurn(s)
	if len(turn) > 0 {
		ro.HasLastTurn = true
		ro.LastTurn = cost(turn, lookup)
		for _, r := range turn {
			extra := r.Usage.Prompt() - ro.Baseline
			if r.Sidechain || extra <= 0 {
				continue
			}
			if rp, ok := lookup(r.Model); ok {
				ro.LastTurnHistory += float64(extra) * rp.CacheRead / 1e6
			}
		}
	}
	ro.Background = cost(background, lookup)

	w := p.CacheWrite5m
	if oneHour {
		w = p.CacheWrite1h
	}
	c, b := float64(ro.Context), float64(ro.Baseline)
	read := func(tokens float64) float64 { return tokens * p.CacheRead / 1e6 }

	ro.ColdNext = c * w / 1e6
	note := float64(HandoverOutputTokens) * p.Output / 1e6
	if ro.Cold {
		ro.Handover = ro.ColdNext + read(c)*(HandoverRequests-1) + note
	} else {
		ro.Handover = read(c)*HandoverRequests + note
	}
	ro.FreshStart = b * w / 1e6
	ro.Fresh = FreshRebuild * b * w / 1e6
	ro.Reset = ro.Handover + ro.Fresh

	if ro.LastTurnHistory > 0 {
		ro.HasPayback = true
		ro.PaybackTurns = ro.Reset / ro.LastTurnHistory
	}
	return ro
}

// lastTurn returns the requests of the last turn that made one, and those that
// background events set off after it. A prompt, or a command that ran, starts
// a turn; a continuation joins the turn before it; a notification's requests
// are background. A command without requests (/clear, /effort) and a prompt
// stopped before any request change nothing.
func lastTurn(s session.Session) (turn, background []session.Request) {
	var current []session.Request
	open := false
	for _, t := range s.Turns {
		switch t.Kind {
		case session.TurnPrompt, session.TurnCommand:
			if len(t.Requests) == 0 {
				continue
			}
			current, background, open = append([]session.Request(nil), t.Requests...), nil, true
		case session.TurnNotification:
			background = append(background, t.Requests...)
		default:
			if open {
				current = append(current, t.Requests...)
			} else {
				background = append(background, t.Requests...)
			}
		}
		if t.Kind == session.TurnNotification {
			open = false
		}
	}
	return current, background
}

func cost(reqs []session.Request, lookup Lookup) float64 {
	var total float64
	for _, r := range reqs {
		if p, ok := lookup(r.Model); ok {
			total += p.Cost(r.Usage)
		}
	}
	return total
}
