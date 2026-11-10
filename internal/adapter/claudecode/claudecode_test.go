package claudecode

import (
	"path/filepath"
	"testing"

	"github.com/Torwalt/turncost/internal/session"
)

func load(t *testing.T) session.Session {
	t.Helper()
	s, err := Load(filepath.Join("testdata", "proj", "s1.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestTurnsStartAtUserTextNotToolResultsOrLocalOutput(t *testing.T) {
	s := load(t)
	var kinds []session.TurnKind
	for _, turn := range s.Turns {
		kinds = append(kinds, turn.Kind)
	}
	want := []session.TurnKind{session.TurnPrompt, session.TurnCommand, session.TurnPrompt, session.TurnNotification}
	if len(kinds) != len(want) {
		t.Fatalf("turn kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("turn kinds = %v, want %v", kinds, want)
		}
	}
	if s.Turns[1].Events[0].Detail != "/effort" {
		t.Errorf("command event = %+v, want /effort", s.Turns[1].Events)
	}
}

func TestRepeatedRequestLinesCountOnce(t *testing.T) {
	first := load(t).Turns[0]
	if len(first.Requests) != 2 {
		t.Fatalf("first turn has %d requests, want 2 (r1 logged twice, r2)", len(first.Requests))
	}
	if got := first.Requests[0].Usage.CacheWrite1h; got != 20_000 {
		t.Errorf("r1 1h write = %d, want 20000", got)
	}
}

func TestCacheWriteWithoutSplitCountsAsFiveMinute(t *testing.T) {
	r2 := load(t).Turns[0].Requests[1]
	if r2.Usage.CacheWrite5m != 500 || r2.Usage.CacheWrite1h != 0 {
		t.Errorf("r2 usage = %+v, want the unsplit write as 5m", r2.Usage)
	}
}

func TestToolUseBecomesEvents(t *testing.T) {
	s := load(t)
	got := map[session.EventKind]int{}
	for _, turn := range s.Turns {
		for _, e := range turn.Events {
			got[e.Kind]++
		}
	}
	for kind, n := range map[session.EventKind]int{session.EventCommit: 1, session.EventPush: 1, session.EventEdit: 1, session.EventPublish: 1} {
		if got[kind] != n {
			t.Errorf("%s events = %d, want %d", kind, got[kind], n)
		}
	}
}

func TestSubagentRequestsJoinTheTurnTheyRanIn(t *testing.T) {
	second := load(t).Turns[2]
	if len(second.Requests) != 2 || !second.Requests[1].Sidechain {
		t.Fatalf("second prompt's requests = %+v, want its own plus the subagent's", second.Requests)
	}
}

func TestSessionMetadataComesFromTheFirstLineThatHasIt(t *testing.T) {
	s := load(t)
	if s.ID != "s1" || s.Project != "/repo" || s.Branch != "master" || s.Harness != Harness {
		t.Errorf("metadata = %+v", s)
	}
}
