package signal

import (
	"math"
	"testing"
	"time"

	"github.com/Torwalt/turncost/internal/pricing"
	"github.com/Torwalt/turncost/internal/session"
)

// opus is the price of the plan's worked example, fixed here so the test does
// not move with the table.
var opus = pricing.Price{Input: 4, Output: 20, CacheRead: 0.20, CacheWrite5m: 5, CacheWrite1h: 8}

func lookup(model string) (pricing.Price, bool) {
	if model == "opus" {
		return opus, true
	}
	return pricing.Price{}, false
}

func main(at time.Time, prompt, write1h int64) session.Request {
	return session.Request{Time: at, Model: "opus", Usage: session.Usage{CacheRead: prompt - write1h, CacheWrite1h: write1h}}
}

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0005 {
		t.Errorf("%s = %.4f, want %.4f", name, got, want)
	}
}

var t0 = time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)

// worked builds the plan's example: B = 48k, C = 450k, one-hour cache, three
// prompts of 20 main requests, the last costing $2.35 with a sidechain request.
func worked() (session.Session, time.Time) {
	group := func(start time.Duration, firstReq, lastReq session.Request) session.Turn {
		reqs := []session.Request{firstReq}
		for i := 1; i < 19; i++ {
			reqs = append(reqs, session.Request{Time: t0.Add(start + time.Duration(i)*time.Second), Model: "opus", Usage: session.Usage{Output: 5_000}})
		}
		return session.Turn{Kind: session.TurnPrompt, Requests: append(reqs, lastReq)}
	}
	filler := func(start time.Duration) session.Request {
		return session.Request{Time: t0.Add(start), Model: "opus", Usage: session.Usage{Output: 5_000}}
	}
	end := t0.Add(30 * time.Minute)
	last := main(end, 450_000, 0)
	// 19 requests × $0.10 + the last one's $0.09 + the sidechain's $0.36.
	side := session.Request{Time: end, Model: "opus", Sidechain: true, Usage: session.Usage{Output: 18_000}}

	t2 := group(20*time.Minute, filler(20*time.Minute), last)
	t2.Requests = append(t2.Requests, side)
	return session.Session{Turns: []session.Turn{
		group(0, main(t0, 48_000, 48_000), filler(time.Minute)),
		group(10*time.Minute, filler(10*time.Minute), filler(11*time.Minute)),
		t2,
	}}, end
}

func TestReadWorkedExampleWarm(t *testing.T) {
	s, at := worked()
	ro := Read(s, at.Add(10*time.Minute), lookup)

	if !ro.HasMain || !ro.Priced || ro.Cold {
		t.Fatalf("readout = %+v, want a warm priced session", ro)
	}
	if ro.Baseline != 48_000 || ro.Context != 450_000 || ro.TTL != time.Hour {
		t.Errorf("B, C, TTL = %d, %d, %v", ro.Baseline, ro.Context, ro.TTL)
	}
	near(t, "last turn", ro.LastTurn, 2.35)
	// Only the last request re-read more than the baseline: (450k − 48k) × $0.20/M.
	near(t, "last turn history", ro.LastTurnHistory, 0.0804)
	near(t, "background", ro.Background, 0)
	near(t, "handover", ro.Handover, 0.37)
	near(t, "fresh start", ro.FreshStart, 0.384)
	near(t, "fresh", ro.Fresh, 0.768)
	near(t, "reset", ro.Reset, 1.138)
	if !ro.HasPayback {
		t.Error("no payback, want one: the last turn carried history")
	}
	near(t, "payback turns", ro.PaybackTurns, 1.138/0.0804)
}

func TestReadWorkedExampleCold(t *testing.T) {
	s, at := worked()
	ro := Read(s, at.Add(72*time.Minute), lookup)

	if !ro.Cold {
		t.Fatal("72 minutes idle on a one-hour cache must be cold")
	}
	near(t, "cold next step", ro.ColdNext, 3.60)
	near(t, "handover", ro.Handover, 3.88)
	near(t, "last turn history", ro.LastTurnHistory, 0.0804)
}

func TestReadHistoryOfALongTurnPaysBackTheReset(t *testing.T) {
	var reqs []session.Request
	reqs = append(reqs, main(t0, 48_000, 48_000))
	turns := []session.Turn{{Kind: session.TurnPrompt, Requests: reqs}}
	var long []session.Request
	for i := 0; i < 10; i++ {
		long = append(long, main(t0.Add(time.Duration(i+1)*time.Minute), 450_000, 0))
	}
	turns = append(turns, session.Turn{Kind: session.TurnPrompt, Requests: long})
	ro := Read(session.Session{Turns: turns}, t0.Add(15*time.Minute), lookup)

	// Ten requests each re-reading 402k beyond the baseline at $0.20/M.
	near(t, "last turn history", ro.LastTurnHistory, 0.804)
	near(t, "last turn", ro.LastTurn, 0.90)
	near(t, "reset", ro.Reset, 1.138)
	near(t, "payback turns", ro.PaybackTurns, 1.138/0.804)
}

func TestReadTurnAndBackground(t *testing.T) {
	cmd := func(reqs ...session.Request) session.Turn {
		return session.Turn{Kind: session.TurnCommand, Requests: reqs}
	}
	prompt := main(t0, 10_000, 10_000)
	cont := main(t0.Add(30*time.Second), 20_000, 0)
	note := main(t0.Add(time.Minute), 30_000, 0)
	next := main(t0.Add(2*time.Minute), 40_000, 0)
	cost := func(rs ...session.Request) float64 {
		var c float64
		for _, r := range rs {
			c += opus.Cost(r.Usage)
		}
		return c
	}
	check := func(name string, ro Readout, turn, background float64) {
		t.Helper()
		if math.Abs(ro.LastTurn-turn) > 1e-9 || math.Abs(ro.Background-background) > 1e-9 {
			t.Errorf("%s: last turn %v background %v, want %v and %v", name, ro.LastTurn, ro.Background, turn, background)
		}
	}

	s := session.Session{Turns: []session.Turn{
		{Kind: session.TurnPrompt, Requests: []session.Request{prompt}},
		{Kind: session.TurnContinuation, Requests: []session.Request{cont}},
		{Kind: session.TurnNotification, Requests: []session.Request{note}},
		cmd(),                      // /clear-like: no requests
		{Kind: session.TurnPrompt}, // stopped before any request
	}}
	check("notification after the reply", Read(s, t0.Add(time.Minute), lookup), cost(prompt, cont), cost(note))

	// A command that ran starts a new turn and clears the background.
	s.Turns = append(s.Turns, cmd(next))
	check("command that ran", Read(s, t0.Add(2*time.Minute), lookup), cost(next), 0)

	// A session that so far only reacted to background events has no turn.
	only := session.Session{Turns: []session.Turn{{Kind: session.TurnNotification, Requests: []session.Request{note}}}}
	ro := Read(only, t0.Add(time.Minute), lookup)
	if ro.HasLastTurn || ro.HasPayback {
		t.Errorf("readout = %+v, want no last turn", ro)
	}
	check("background only", ro, 0, cost(note))
}

func TestReadSkipsGroupsWithoutRequests(t *testing.T) {
	r := main(t0, 10_000, 10_000)
	s := session.Session{Turns: []session.Turn{
		{Kind: session.TurnPrompt, Requests: []session.Request{r}},
		{Kind: session.TurnPrompt}, // aborted before any request
	}}
	ro := Read(s, t0, lookup)
	if want := opus.Cost(r.Usage); math.Abs(ro.LastTurn-want) > 1e-9 {
		t.Errorf("last turn = %v, want the last group with a request (%v)", ro.LastTurn, want)
	}
}

func TestReadUnknownModelCarriesContextOnly(t *testing.T) {
	r := main(t0, 90_000, 90_000)
	r.Model = "mystery"
	ro := Read(session.Session{Turns: []session.Turn{{Kind: session.TurnPrompt, Requests: []session.Request{r}}}}, t0, lookup)
	want := Readout{
		HasMain: true, Baseline: 90_000, Context: 90_000, TTL: time.Hour,
		LastRequest: t0, Idle: 0,
	}
	if ro != want {
		t.Errorf("readout = %+v, want only the context: %+v", ro, want)
	}
}

func TestReadNoMainRequest(t *testing.T) {
	side := main(t0, 90_000, 0)
	side.Sidechain = true
	for _, s := range []session.Session{
		{},
		{Turns: []session.Turn{{Kind: session.TurnPrompt}}},
		{Turns: []session.Turn{{Kind: session.TurnPrompt, Requests: []session.Request{side}}}},
	} {
		if ro := Read(s, t0, lookup); ro != (Readout{}) {
			t.Errorf("readout = %+v, want empty without a main request", ro)
		}
	}
}

func TestReadNoHistoryHasNoPayback(t *testing.T) {
	// One request: the context is the baseline.
	s := session.Session{Turns: []session.Turn{{Kind: session.TurnPrompt, Requests: []session.Request{main(t0, 48_000, 48_000)}}}}
	ro := Read(s, t0, lookup)
	if ro.LastTurnHistory != 0 || ro.HasPayback || ro.PaybackTurns != 0 {
		t.Errorf("readout = %+v, want no history and no payback", ro)
	}

	// Context below the baseline (after a compaction) is no history, not negative.
	s.Turns = append(s.Turns, session.Turn{Kind: session.TurnPrompt, Requests: []session.Request{main(t0.Add(time.Minute), 30_000, 0)}})
	if ro := Read(s, t0.Add(time.Minute), lookup); ro.LastTurnHistory != 0 || ro.HasPayback {
		t.Errorf("history = %v payback = %v below the baseline", ro.LastTurnHistory, ro.HasPayback)
	}
}

func TestReadFiveMinuteTTLWithoutOneHourWrites(t *testing.T) {
	r := session.Request{Time: t0, Model: "opus", Usage: session.Usage{CacheRead: 40_000, CacheWrite5m: 10_000}}
	s := session.Session{Turns: []session.Turn{{Kind: session.TurnPrompt, Requests: []session.Request{r}}}}

	warm := Read(s, t0.Add(4*time.Minute), lookup)
	if warm.TTL != 5*time.Minute || warm.Cold {
		t.Errorf("ttl = %v cold = %v at 4 minutes", warm.TTL, warm.Cold)
	}
	cold := Read(s, t0.Add(6*time.Minute), lookup)
	if !cold.Cold {
		t.Error("6 minutes idle on a five-minute cache must be cold")
	}
	near(t, "cold next step", cold.ColdNext, 50_000*5/1e6)
}

func TestReadIsPure(t *testing.T) {
	s, at := worked()
	a := Read(s, at, lookup)
	b := Read(s, at, lookup)
	if a != b {
		t.Errorf("same input gave %+v and %+v", a, b)
	}
}
