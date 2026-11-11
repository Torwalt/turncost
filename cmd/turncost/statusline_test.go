package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
		{"warm", warm, warmNow, "450k · turn $0.27 (hist $0.08)\n"},
		{"cold", warm, coldNow, "450k · turn $0.27 (hist $0.08) · cold +$3.60\n"},
		{"background after the reply", payload(t, "testdata/background.jsonl"), warmNow, "450k · turn $0.27 (hist $0.08) · bg +$0.09\n"},
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
	base := signal.Readout{HasMain: true, Priced: true, Baseline: 48_000, Context: 241_000, HasLastTurn: true, LastTurn: 1.75}
	tests := []struct {
		name string
		edit func(*signal.Readout)
		want string
	}{
		{"no history", func(*signal.Readout) {}, "241k · turn $1.75"},
		{"history", func(r *signal.Readout) { r.LastTurnHistory = 0.71 }, "241k · turn $1.75 (hist $0.71)"},
		{"unknown model", func(r *signal.Readout) { r.Priced = false }, "241k"},
		{"no last turn", func(r *signal.Readout) { r.HasLastTurn = false }, "241k"},
		{"background only", func(r *signal.Readout) { r.HasLastTurn, r.Background = false, 0.4 }, "241k · bg +$0.40"},
		{"background", func(r *signal.Readout) { r.Background = 0.4 }, "241k · turn $1.75 · bg +$0.40"},
		{"reset pays", func(r *signal.Readout) { r.LastTurnHistory, r.ResetPays = 2.47, true }, "241k · turn $1.75 (hist $2.47) · ↻ reset pays"},
		{"cold", func(r *signal.Readout) { r.Cold, r.ColdNext = true, 4.78 }, "241k · turn $1.75 · cold +$4.78"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ro := base
			tc.edit(&ro)
			if got := segment(ro, fmt.Sprintf("%dk", ro.Context/1000), false); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
	if got := segment(signal.Readout{}, "", false); got != "" {
		t.Errorf("segment of no main request = %q, want empty", got)
	}
}

func TestSegmentColoursOnlyTheNudgeAndTheColdPrice(t *testing.T) {
	ro := signal.Readout{HasMain: true, Priced: true, Context: 571_000, HasLastTurn: true, LastTurn: 3.26, LastTurnHistory: 2.47, ResetPays: true}
	if got, want := segment(ro, fmt.Sprintf("%dk", ro.Context/1000), true), "571k · turn $3.26 (hist $2.47) · \x1b[1;33m↻ reset pays\x1b[0m"; got != want {
		t.Errorf("nudge: got %q, want %q", got, want)
	}
	ro.ResetPays, ro.Cold, ro.ColdNext = false, true, 4.57
	if got, want := segment(ro, fmt.Sprintf("%dk", ro.Context/1000), true), "571k · turn $3.26 (hist $2.47) · \x1b[36mcold +$4.57\x1b[0m"; got != want {
		t.Errorf("cold: got %q, want %q", got, want)
	}
	ro.Cold = false
	if got := segment(ro, fmt.Sprintf("%dk", ro.Context/1000), true); strings.Contains(got, "\x1b") {
		t.Errorf("plain state is coloured: %q", got)
	}
}

func TestContextShowsTheShareLeftWhenTheHarnessReportsIt(t *testing.T) {
	warm := payload(t, "testdata/warm.jsonl")
	withLeft := strings.Replace(warm, "{", `{"context_window":{"remaining_percentage":74.6},`, 1)
	if got, want := statusline(t, withLeft, warmNow), "ctx 74% · turn $0.27 (hist $0.08)\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	withNull := strings.Replace(warm, "{", `{"context_window":{"remaining_percentage":null},`, 1)
	if got, want := statusline(t, withNull, warmNow), "450k · turn $0.27 (hist $0.08)\n"; got != want {
		t.Errorf("null share: got %q, want %q", got, want)
	}
}

func TestStatuslineHonoursNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if got := statusline(t, payload(t, "testdata/warm.jsonl"), coldNow); !strings.Contains(got, "\x1b[36m") {
		t.Errorf("without NO_COLOR the cold price is plain: %q", got)
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
	seg := "450k · turn $0.27 (hist $0.08)\n"
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
