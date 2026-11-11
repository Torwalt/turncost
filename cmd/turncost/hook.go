package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Torwalt/turncost/internal/adapter"
	"github.com/Torwalt/turncost/internal/handover"
	"github.com/Torwalt/turncost/internal/pricing"
	"github.com/Torwalt/turncost/internal/signal"
)

// guardCarry is how far the context must have grown above the baseline, in
// tokens, before the cold guard warns. A starting value chosen while writing
// the plan, not measured.
const guardCarry = 100_000

// runHook runs a harness hook. Only `prompt` exists; it prints a block decision
// or nothing, and exits 0 whatever goes wrong.
func runHook(args []string, stdin io.Reader, w, errw io.Writer, now time.Time) int {
	if len(args) != 1 || args[0] != "prompt" {
		fmt.Fprintln(errw, "usage: turncost hook prompt")
		return 2
	}
	input, _ := io.ReadAll(stdin)
	reason, err := coldGuard(input, now)
	if err != nil {
		fmt.Fprintln(errw, "turncost:", err)
		return 0
	}
	if reason == "" {
		return 0
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]string{"decision": "block", "reason": reason}); err != nil {
		fmt.Fprintln(errw, "turncost:", err)
		return 0
	}
	w.Write(buf.Bytes())
	return 0
}

// coldGuard returns the reason to block a prompt, or "" to let it through. It
// blocks a handover typed into a cold session with a large context once per
// session and last request, and records that it did.
func coldGuard(input []byte, now time.Time) (string, error) {
	harness := adapter.Default()
	ref, prompt, err := harness.ParsePrompt(input)
	if err != nil {
		return "", err
	}
	if !isHandover(prompt) || ref.Transcript == "" || !markable(ref.ID) {
		return "", nil
	}
	s, err := harness.Load(ref.Transcript)
	if err != nil {
		return "", err
	}
	ro := signal.Read(s, now, pricing.Lookup)
	if !ro.HasMain || !ro.Priced || !ro.Cold || ro.Context-ro.Baseline < guardCarry {
		return "", nil
	}

	marker := filepath.Join(handover.StateDir(), "warned", ref.ID)
	stamp := ro.LastRequest.UTC().Format(time.RFC3339)
	if b, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(b)) == stamp {
		return "", nil
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return "", err
	}
	// A marker that cannot be written must not block: the user could never
	// get through by sending the prompt again.
	if err := os.WriteFile(marker, []byte(stamp+"\n"), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("turncost: the cache is cold (idle %s). This handover first re-writes %dk tokens ≈ $%.2f; a fresh session starts at ≈ $%.2f. Send /handover again to do it anyway, or /clear and start from git.",
		idleText(ro.Idle), ro.Context/1000, ro.ColdNext, ro.FreshStart), nil
}

// isHandover reports whether the prompt is the handover command, alone or
// followed by whitespace and arguments.
func isHandover(prompt string) bool {
	prompt = strings.TrimSpace(prompt)
	for _, name := range []string{"/handover", "/turncost:handover"} {
		rest, ok := strings.CutPrefix(prompt, name)
		if ok && (rest == "" || strings.TrimLeft(rest, " \t\r\n") != rest) {
			return true
		}
	}
	return false
}

// markable reports whether a session id is usable as a file name.
func markable(id string) bool {
	return id != "" && id != "." && id != ".." && id == filepath.Base(id)
}

// idleText prints a duration rounded to the minute as 1h12m or 45m.
func idleText(d time.Duration) string {
	m := int(d.Round(time.Minute) / time.Minute)
	if m >= 60 {
		return fmt.Sprintf("%dh%dm", m/60, m%60)
	}
	return fmt.Sprintf("%dm", m)
}
