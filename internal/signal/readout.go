package signal

import (
	"math"
	"sort"
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
// its next step costs and when a reset would pay. All figures are dollars or
// tokens; the zero value of a Has flag means the figure is absent.
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

	HasLastTurn  bool
	LastTurn     float64
	StepsPerTurn float64

	Step         float64
	Carry        float64
	HistoryShare float64
	ColdNext     float64
	Handover     float64
	FreshStart   float64
	Fresh        float64
	Reset        float64

	// HasPayback is false when the carry is zero.
	HasPayback   bool
	PaybackSteps int
	PaybackTurns float64
}

// Read computes the readout of a session at time now. It is pure: same
// session, time and prices, same figures.
func Read(s session.Session, now time.Time, lookup Lookup) Readout {
	var first, last *session.Request
	oneHour := false
	groups := groupTurns(s)
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

	var steps []int
	for _, g := range groups {
		if g.requests > 0 {
			ro.HasLastTurn = true
			ro.LastTurn = g.cost(lookup)
		}
		if g.main > 0 {
			steps = append(steps, g.main)
		}
	}
	ro.StepsPerTurn = median(steps)

	w := p.CacheWrite5m
	if oneHour {
		w = p.CacheWrite1h
	}
	c, b := float64(ro.Context), float64(ro.Baseline)
	read := func(tokens float64) float64 { return tokens * p.CacheRead / 1e6 }

	ro.Step = read(c)
	ro.Carry = read(math.Max(c-b, 0))
	if ro.Step > 0 {
		ro.HistoryShare = ro.Carry / ro.Step
	}
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

	if ro.Carry > 0 {
		ro.HasPayback = true
		ro.PaybackSteps = int(math.Ceil(ro.Reset / ro.Carry))
		if ro.StepsPerTurn > 0 {
			ro.PaybackTurns = float64(ro.PaybackSteps) / ro.StepsPerTurn
		}
	}
	return ro
}

// group is a user turn: a prompt or a command that ran, with the
// notifications and continuations that followed it.
type group struct {
	reqs     []session.Request
	requests int
	main     int
}

func (g group) cost(lookup Lookup) float64 {
	var total float64
	for _, r := range g.reqs {
		if p, ok := lookup(r.Model); ok {
			total += p.Cost(r.Usage)
		}
	}
	return total
}

// groupTurns folds turns into user turns. A command without requests (/clear,
// /effort) starts nothing and joins nothing. A notification or continuation
// with nothing before it to join starts a group of its own, so its requests
// are not lost.
func groupTurns(s session.Session) []group {
	var groups []group
	for _, t := range s.Turns {
		switch t.Kind {
		case session.TurnPrompt:
			groups = append(groups, group{})
		case session.TurnCommand:
			if len(t.Requests) == 0 {
				continue
			}
			groups = append(groups, group{})
		default:
			if len(groups) == 0 {
				groups = append(groups, group{})
			}
		}
		g := &groups[len(groups)-1]
		for _, r := range t.Requests {
			g.reqs = append(g.reqs, r)
			g.requests++
			if !r.Sidechain {
				g.main++
			}
		}
	}
	return groups
}

// median of the counts; the mean of the two middle ones when even; 0 for none.
func median(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	sort.Ints(xs)
	n := len(xs)
	if n%2 == 1 {
		return float64(xs[n/2])
	}
	return float64(xs[n/2-1]+xs[n/2]) / 2
}
