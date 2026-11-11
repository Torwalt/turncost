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
)

const guardReason = "turncost: the cache is cold (idle 1h12m). This handover first re-writes 450k tokens ≈ $3.60; a fresh session starts at ≈ $0.38. Send /handover again to do it anyway, or /clear and start from git."

func promptPayload(t *testing.T, transcript, id, prompt string) string {
	t.Helper()
	abs, err := filepath.Abs(transcript)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(map[string]any{
		"session_id": id, "transcript_path": abs, "cwd": "/repo",
		"hook_event_name": "UserPromptSubmit", "prompt": prompt,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func hook(t *testing.T, input string, now time.Time) (string, int) {
	t.Helper()
	var out, errw bytes.Buffer
	code := runHook([]string{"prompt"}, strings.NewReader(input), &out, &errw, now)
	return out.String(), code
}

func blockOf(reason string) string {
	b, _ := json.Marshal(map[string]string{"decision": "block", "reason": reason})
	return strings.ReplaceAll(string(b), `<`, "<") + "\n"
}

// transcript writes a one-hour-cache session whose first main request has a
// prompt of base tokens and whose last, at 10:30:19Z, has one of context.
func transcript(t *testing.T, base, context int) string {
	t.Helper()
	line := func(id string, at string, prompt int) string {
		return fmt.Sprintf(`{"type": "assistant", "sessionId": "g1", "requestId": %q, "timestamp": %q, "message": {"id": "m%s", "model": "claude-opus-5-5", "content": [{"type": "text", "text": "ok"}], "usage": {"input_tokens": 0, "cache_read_input_tokens": 0, "cache_creation_input_tokens": %d, "output_tokens": 10, "cache_creation": {"ephemeral_5m_input_tokens": 0, "ephemeral_1h_input_tokens": %d}}}}`,
			id, at, id, prompt, prompt)
	}
	body := `{"type": "user", "sessionId": "g1", "cwd": "/repo", "version": "2.1.0", "timestamp": "2026-11-11T10:00:00Z", "message": {"role": "user", "content": "go"}}` + "\n" +
		line("a", "2026-11-11T10:00:01Z", base) + "\n" + line("b", "2026-11-11T10:30:19Z", context) + "\n"
	path := filepath.Join(t.TempDir(), "g1.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestColdGuardBlocksOnceThenLetsTheSamePromptThrough(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	in := promptPayload(t, "testdata/warm.jsonl", "w1", "/handover")

	got, code := hook(t, in, coldNow)
	if want := blockOf(guardReason); got != want || code != 0 {
		t.Fatalf("first send: got %q (exit %d), want %q", got, code, want)
	}
	marker, err := os.ReadFile(filepath.Join(os.Getenv("XDG_STATE_HOME"), "turncost", "warned", "w1"))
	if err != nil || strings.TrimSpace(string(marker)) != "2026-11-11T10:30:19Z" {
		t.Errorf("marker = %q, %v; want the last request time", marker, err)
	}
	if got, code := hook(t, in, coldNow); got != "" || code != 0 {
		t.Errorf("second send: got %q (exit %d), want nothing", got, code)
	}
}

func TestColdGuardBlocksAgainAfterANewRequest(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := filepath.Join(os.Getenv("XDG_STATE_HOME"), "turncost", "warned")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "w1"), []byte("2026-11-10T09:00:00Z\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := hook(t, promptPayload(t, "testdata/warm.jsonl", "w1", "/handover"), coldNow); got == "" {
		t.Error("a marker for an earlier last request let the prompt through")
	}
}

func TestColdGuardPasses(t *testing.T) {
	small := transcript(t, 48_000, 147_000) // C − B just under the threshold
	edge := transcript(t, 48_000, 48_000+guardCarry)
	tests := []struct {
		name  string
		input string
		now   time.Time
		block bool
	}{
		{"warm", promptPayload(t, "testdata/warm.jsonl", "w1", "/handover"), warmNow, false},
		{"small carry", promptPayload(t, small, "g1", "/handover"), coldNow, false},
		{"carry at the threshold", promptPayload(t, edge, "g2", "/handover"), coldNow, true},
		{"other prompt", promptPayload(t, "testdata/warm.jsonl", "w1", "fix the bug"), coldNow, false},
		{"handoverx", promptPayload(t, "testdata/warm.jsonl", "w1", "/handoverx"), coldNow, false},
		{"please /handover", promptPayload(t, "testdata/warm.jsonl", "w1", "please /handover"), coldNow, false},
		{"takeover", promptPayload(t, "testdata/warm.jsonl", "w1", "/takeover"), coldNow, false},
		{"unreadable transcript", promptPayload(t, "testdata/absent.jsonl", "w1", "/handover"), coldNow, false},
		{"no transcript", `{"session_id":"w1","prompt":"/handover"}`, coldNow, false},
		{"no session id", promptPayload(t, "testdata/warm.jsonl", "", "/handover"), coldNow, false},
		{"session id that is a path", promptPayload(t, "testdata/warm.jsonl", "../x", "/handover"), coldNow, false},
		{"not json", `nonsense`, coldNow, false},
		{"plugin-qualified", promptPayload(t, "testdata/warm.jsonl", "w1", "/turncost:handover"), coldNow, true},
		{"with arguments", promptPayload(t, "testdata/warm.jsonl", "w1", "  /handover for the next dev\n"), coldNow, true},
		{"newline before arguments", promptPayload(t, "testdata/warm.jsonl", "w1", "/handover\nmore"), coldNow, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			got, code := hook(t, tc.input, tc.now)
			if code != 0 {
				t.Errorf("exit %d, want 0", code)
			}
			if blocked := got != ""; blocked != tc.block {
				t.Errorf("output %q, blocked = %v, want %v", got, blocked, tc.block)
			}
			if tc.block && !strings.Contains(got, `"decision":"block"`) {
				t.Errorf("output %q is not a block decision", got)
			}
		})
	}
}

func TestIdleText(t *testing.T) {
	for d, want := range map[time.Duration]string{
		72 * time.Minute:               "1h12m",
		45 * time.Minute:               "45m",
		60 * time.Minute:               "1h0m",
		5*time.Minute + 40*time.Second: "6m",
		26*time.Hour + 3*time.Minute:   "26h3m",
	} {
		if got := idleText(d); got != want {
			t.Errorf("idleText(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestHookRejectsUnknownSubcommand(t *testing.T) {
	var out, errw bytes.Buffer
	if code := runHook([]string{"nope"}, strings.NewReader(""), &out, &errw, coldNow); code != 2 || out.Len() != 0 {
		t.Errorf("exit %d, output %q; want exit 2 and no output", code, out.String())
	}
}
