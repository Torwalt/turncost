package signal

import (
	"time"

	"github.com/Torwalt/turncost/internal/pricing"
	"github.com/Torwalt/turncost/internal/session"
)

// The reset estimate and the nudge. The first three are fitted to the
// author's seven handovers of 2026-11-12 to 2026-11-18, the last to a backtest
// of 102 interactive sessions of 2026-11-11 to 2026-11-19.
const (
	// HandoverRequests is how many cache-read passes over the context writing
	// a handover note takes (cold: the first one is a cache write).
	HandoverRequests = 4
	// HandoverOutputTokens is the length of the handover note.
	HandoverOutputTokens = 5000
	// FreshRebuild is how many baselines of cache write a fresh session pays
	// to take the note over.
	FreshRebuild = 1.2
	// RemainingRequests is how many more main requests the nudge expects the
	// session to make. Sessions run a median of about 40 more after a prompt at
	// 200k of carried context; 20 keeps the nudge from firing near their end.
	RemainingRequests = 20
)

// Lookup finds the price of a model; pricing.Lookup is the table the program
// uses.
type Lookup func(model string) (pricing.Price, bool)

// Readout is what the live surfaces show about a session: its context, what
// the last turn cost, what each further request pays for the context carried
// and what a reset would cost. All figures are dollars or tokens; the zero
// value of a Has flag means the figure is absent.
//
// A turn is the user's message and everything it set off up to the next one:
// the agent's reply, its subagents and its reactions to background events.
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

	// LastTurn is what the last turn with a request cost, subagents and
	// background follow-ups included.
	HasLastTurn bool
	LastTurn    float64

	// Carry is what the next main request pays to re-read the context beyond
	// the baseline, at cache-read price: what a reset saves on every request.
	Carry float64

	ColdNext   float64
	Handover   float64
	FreshStart float64
	Fresh      float64
	Reset      float64

	// Saving is what a reset now saves over the next RemainingRequests main
	// requests: their Carry less the reset. It rises with the context, since
	// the carry grows by RemainingRequests reads of every new token and the
	// reset by HandoverRequests.
	Saving float64

	// ResetPays is the nudge: Saving is not negative. Never set while cold:
	// then the next turn's re-write is the figure that matters.
	ResetPays bool
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

	if turn := lastTurn(s); len(turn) > 0 {
		ro.HasLastTurn = true
		ro.LastTurn = cost(turn, lookup)
	}

	w := p.CacheWrite5m
	if oneHour {
		w = p.CacheWrite1h
	}
	c, b := float64(ro.Context), float64(ro.Baseline)
	read := func(tokens float64) float64 { return tokens * p.CacheRead / 1e6 }

	ro.Carry = read(max(c-b, 0))
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
	ro.Saving = ro.Carry*RemainingRequests - ro.Reset
	ro.ResetPays = !ro.Cold && ro.Carry > 0 && ro.Saving >= 0
	return ro
}

// lastTurn returns the requests of the last turn that made one. A prompt, or a
// command that ran, starts a turn; everything after it up to the next one joins
// it, background events and the subagent work timed into them included. A
// command without requests (/clear, /effort) and a prompt stopped before any
// request change nothing.
func lastTurn(s session.Session) []session.Request {
	var current []session.Request
	for _, t := range s.Turns {
		if (t.Kind == session.TurnPrompt || t.Kind == session.TurnCommand) && len(t.Requests) > 0 {
			current = nil
		}
		current = append(current, t.Requests...)
	}
	return current
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
