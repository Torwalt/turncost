package signal

import (
	"math"
	"testing"
	"time"

	"github.com/Torwalt/turncost/internal/session"
)

func req(at time.Time, read, write1h, out int64) session.Request {
	return session.Request{Time: at, Model: "claude-opus-5-5", Usage: session.Usage{CacheRead: read, CacheWrite1h: write1h, Output: out}}
}

func TestAnalyzeAccumulatesTurnCosts(t *testing.T) {
	t0 := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	s := session.Session{Turns: []session.Turn{
		{Kind: session.TurnPrompt, Requests: []session.Request{req(t0, 0, 20_000, 1_000)}},
		{Kind: session.TurnPrompt, Requests: []session.Request{req(t0.Add(time.Minute), 120_000, 0, 1_000)}},
	}}
	sum := Analyze(s)

	if sum.Baseline != 20_000 {
		t.Fatalf("baseline = %d, want the first request's prompt", sum.Baseline)
	}
	first := 20_000*8/1e6 + 1_000*20/1e6
	second := 120_000*0.2/1e6 + 1_000*20/1e6
	if got := sum.Turns[1].Cumulative; math.Abs(got-(first+second)) > 1e-9 {
		t.Errorf("cumulative = %v, want %v", got, first+second)
	}
	if got, want := sum.Turns[1].HistoryCost, 100_000*0.2/1e6; math.Abs(got-want) > 1e-9 {
		t.Errorf("history cost = %v, want %v (tokens above baseline at read price)", got, want)
	}
}

func TestGapPastTTLMarksColdCache(t *testing.T) {
	t0 := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	s := session.Session{Turns: []session.Turn{
		{Requests: []session.Request{req(t0, 0, 10_000, 10), req(t0.Add(time.Minute), 10_000, 0, 10)}},
		{Requests: []session.Request{req(t0.Add(62*time.Minute), 0, 50_000, 10)}},
	}}
	sum := Analyze(s)
	if !sum.Turns[1].Cold {
		t.Error("a turn after more than an hour on a 1h cache must be cold")
	}
	if sum.Turns[0].Cold || sum.Turns[0].Gap != 0 {
		t.Error("the first turn has no previous request and has no gap")
	}
}

func TestSidechainCostCountsButNotContext(t *testing.T) {
	t0 := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	sub := req(t0.Add(time.Second), 500_000, 0, 0)
	sub.Sidechain = true
	s := session.Session{Turns: []session.Turn{{Requests: []session.Request{req(t0, 0, 10_000, 0), sub}}}}
	sum := Analyze(s)
	if sum.Peak != 10_000 {
		t.Errorf("peak = %d, want the main thread's context only", sum.Peak)
	}
	if sum.Cost <= 10_000*8/1e6 {
		t.Error("subagent cost must count toward the turn")
	}
}
