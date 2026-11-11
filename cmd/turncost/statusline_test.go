package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Torwalt/turncost/internal/signal"
)

// The last main request of testdata/warm.jsonl is at 10:30:19Z; its cache is
// one-hour, B = 48k, C = 450k, two prompts of 20 main requests each.
var (
	warmNow = time.Date(2026, 11, 11, 10, 40, 19, 0, time.UTC)
	coldNow = time.Date(2026, 11, 11, 11, 42, 19, 0, time.UTC)
)

func payload(t *testing.T, transcript string) string {
	t.Helper()
	abs, err := filepath.Abs(transcript)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(map[string]any{
		"session_id": "w1", "transcript_path": abs, "cwd": "/repo",
		"model": map[string]string{"id": "claude-opus-5-5"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func statusline(t *testing.T, input string, now time.Time, args ...string) string {
	t.Helper()
	var out, errw bytes.Buffer
	runStatusline(args, strings.NewReader(input), &out, &errw, now)
	return out.String()
}

func TestSegmentWarmColdAndEmpty(t *testing.T) {
	warm := payload(t, "testdata/warm.jsonl")
	tests := []struct {
		name  string
		input string
		now   time.Time
		want  string
	}{
		{"warm", warm, warmNow, "450k · last turn $0.27 · step $0.090 (89% history) · reset pays in ~15 steps (<1 turn)\n"},
		{"cold", warm, coldNow, "450k · cold: next step $3.60 · last turn $0.27\n"},
		{"no main request", payload(t, "testdata/empty.jsonl"), warmNow, ""},
		{"missing transcript", payload(t, "testdata/absent.jsonl"), warmNow, ""},
		{"no transcript path", `{"session_id":"w1"}`, warmNow, ""},
		{"empty object", `{}`, warmNow, ""},
		{"not json", `nonsense`, warmNow, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusline(t, tc.input, tc.now); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSegmentShapes(t *testing.T) {
	base := signal.Readout{HasMain: true, Priced: true, Baseline: 48_000, Context: 48_000, HasLastTurn: true, LastTurn: 1.5, Step: 0.0096}
	tests := []struct {
		name string
		edit func(*signal.Readout)
		want string
	}{
		{"no carry", func(*signal.Readout) {}, "48k · last turn $1.50 · step $0.010"},
		{"unknown model", func(r *signal.Readout) { r.Priced = false }, "48k"},
		{"no last turn", func(r *signal.Readout) { r.HasLastTurn = false }, "48k · step $0.010"},
		{"cold without last turn", func(r *signal.Readout) { r.Cold, r.ColdNext, r.HasLastTurn = true, 0.38, false }, "48k · cold: next step $0.38"},
		{"one turn", func(r *signal.Readout) {
			r.HasPayback, r.PaybackSteps, r.PaybackTurns, r.HistoryShare = true, 20, 1.2, 0.5
		}, "48k · last turn $1.50 · step $0.010 (50% history) · reset pays in ~20 steps (~1 turn)"},
		{"several turns", func(r *signal.Readout) {
			r.HasPayback, r.PaybackSteps, r.PaybackTurns, r.HistoryShare = true, 1, 2.5, 0.5
		}, "48k · last turn $1.50 · step $0.010 (50% history) · reset pays in ~1 step (~3 turns)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ro := base
			tc.edit(&ro)
			if got := segment(ro); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
	if got := segment(signal.Readout{}); got != "" {
		t.Errorf("segment of no main request = %q, want empty", got)
	}
}

func script(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "theirs.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWrappedCommandJoinsTheSegment(t *testing.T) {
	warm := payload(t, "testdata/warm.jsonl")
	seg := "450k · last turn $0.27 · step $0.090 (89% history) · reset pays in ~15 steps (<1 turn)\n"
	tests := []struct {
		name  string
		input string
		body  string
		want  string
	}{
		{"output with newline", warm, `printf 'main *\n'`, "main * · " + seg},
		{"output without newline", warm, `printf 'main *'`, "main * · " + seg},
		{"several trailing newlines", warm, `printf 'main *\n\n\n'`, "main * · " + seg},
		{"multi-line output joins its last line", warm, `printf 'one\ntwo\n'`, "one\ntwo · " + seg},
		{"prints nothing", warm, `exit 0`, seg},
		{"fails after printing", warm, `printf 'main *\n'; exit 3`, "main * · " + seg},
		{"fails silently", warm, `exit 3`, seg},
		{"empty segment", `{}`, `printf 'main *\n'`, "main *\n"},
		{"both empty", `{}`, `exit 0`, ""},
		{"gets the same stdin", `{"x":"hello"}`, `cat; echo`, `{"x":"hello"}` + "\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusline(t, tc.input, warmNow, "--", script(t, tc.body)); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWrappedCommandThatDoesNotExistIsIgnored(t *testing.T) {
	warm := payload(t, "testdata/warm.jsonl")
	got := statusline(t, warm, warmNow, "--", filepath.Join(t.TempDir(), "absent"))
	if !strings.HasPrefix(got, "450k · ") {
		t.Errorf("got %q, want the segment alone", got)
	}
}

func TestStatuslineIsPureOverItsInput(t *testing.T) {
	warm := payload(t, "testdata/warm.jsonl")
	if a, b := statusline(t, warm, warmNow), statusline(t, warm, warmNow); a != b {
		t.Errorf("two runs differ: %q vs %q", a, b)
	}
}
