package claudecode

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestCurrentIsTheNewestTranscriptOfTheEncodedProject(t *testing.T) {
	root := t.TempDir()
	dir := "/home/ada/repos/art-deal-flow.worktrees/capture-flow"
	enc := "-home-ada-repos-art-deal-flow-worktrees-capture-flow"
	if got := EncodeDir(dir); got != enc {
		t.Fatalf("EncodeDir(%q) = %q, want %q", dir, got, enc)
	}
	project := filepath.Join(root, enc)
	other := filepath.Join(root, "-home-ada-repos-art-deal-flow-worktrees-other")
	for _, d := range []string{project, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	files := map[string]time.Time{
		filepath.Join(project, "old.jsonl"):  now.Add(-2 * time.Hour),
		filepath.Join(project, "new.jsonl"):  now.Add(-time.Hour),
		filepath.Join(other, "newest.jsonl"): now,
		filepath.Join(project, "notes.txt"):  now,
	}
	for p, mod := range files {
		if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Current(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(project, "new.jsonl"); got != want {
		t.Errorf("Current = %q, want %q", got, want)
	}
	if _, err := Current(root, "/home/ada/nowhere"); err == nil {
		t.Error("Current of a directory without transcripts succeeded, want an error")
	}
}

func TestParseStatuslineReadsTheSessionItNames(t *testing.T) {
	ref, err := ParseStatusline([]byte(`{"session_id":"s1","transcript_path":"/t/s1.jsonl","cwd":"/repo","model":{"id":"x"}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := session.Ref{ID: "s1", Transcript: "/t/s1.jsonl", Dir: "/repo"}
	if ref != want {
		t.Errorf("ref = %+v, want %+v", ref, want)
	}
	if _, err := ParseStatusline([]byte(`not json`)); err == nil {
		t.Error("malformed payload parsed without error")
	}
	if ref, err := ParseStatusline([]byte(`{}`)); err != nil || ref != (session.Ref{}) {
		t.Errorf("empty payload = %+v, %v; want the zero ref", ref, err)
	}
}
