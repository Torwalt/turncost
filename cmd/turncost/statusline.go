package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Torwalt/turncost/internal/adapter"
	"github.com/Torwalt/turncost/internal/handover"
	"github.com/Torwalt/turncost/internal/pricing"
	"github.com/Torwalt/turncost/internal/signal"
)

// runStatusline prints the statusline segment for the session the harness
// names on stdin, after the output of the command in args (`-- cmd args…`), if
// any. It never fails: whatever goes wrong, it prints what it has.
func runStatusline(args []string, stdin io.Reader, w, errw io.Writer, now time.Time) {
	input, _ := io.ReadAll(stdin)
	var command []string
	switch {
	case len(args) == 0:
	case args[0] == "--":
		command = args[1:]
	default:
		fmt.Fprintln(errw, "turncost: statusline takes only [-- command args…]")
	}

	var theirs string
	if len(command) > 0 {
		theirs = runWrapped(command, input, errw)
	}
	seg, err := statuslineSegment(input, now)
	if err != nil {
		fmt.Fprintln(errw, "turncost:", err)
	}

	switch {
	case theirs != "" && seg != "":
		fmt.Fprintf(w, "%s · %s\n", theirs, seg)
	case theirs != "":
		fmt.Fprintln(w, theirs)
	case seg != "":
		fmt.Fprintln(w, seg)
	}
}

// runWrapped runs the user's own statusline command with the same input and
// returns its output without trailing newlines. Its failure is ignored.
func runWrapped(command []string, input []byte, errw io.Writer) string {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stderr = errw
	out, _ := cmd.Output()
	return strings.TrimRight(string(out), "\n")
}

// statuslineSegment is the segment for the session the statusline input names.
// A session with no main request yet, or whose transcript is not there yet,
// shows the handover note waiting for its project, if any, else nothing.
func statuslineSegment(input []byte, now time.Time) (string, error) {
	harness := adapter.Default()
	ref, err := harness.ParseStatusline(input)
	if err != nil {
		return "", err
	}
	waiting := func() string { return waitingSegment(harness, ref.Dir) }
	if ref.Transcript == "" {
		return waiting(), nil
	}
	s, err := harness.Load(ref.Transcript)
	if err != nil {
		if os.IsNotExist(err) {
			return waiting(), nil
		}
		return "", err
	}
	ro := signal.Read(s, now, pricing.Lookup)
	if !ro.HasMain {
		return waiting(), nil
	}
	return segment(ro, os.Getenv("NO_COLOR") == ""), nil
}

// waitingSegment names the handover note waiting for the project of dir.
func waitingSegment(harness adapter.Adapter, dir string) string {
	if dir == "" {
		return ""
	}
	n, ok, err := handover.Default().Waiting(harness.ProjectKey(dir))
	if err != nil || !ok {
		return ""
	}
	return "handover waiting since " + n.Written.Local().Format("15:04") + " · /takeover"
}

// ANSI colours for the two states that ask for attention.
const (
	colorReset = "\x1b[0m"
	colorNudge = "\x1b[1;33m" // bold yellow
	colorCold  = "\x1b[36m"   // cyan
)

// segment formats a readout as the statusline text, short enough to share a
// line with the user's own statusline: context, the last turn's cost and its
// history, background follow-ups when any, then the cold price or the nudge.
// color adds ANSI colour to the last two.
func segment(ro signal.Readout, color bool) string {
	if !ro.HasMain {
		return ""
	}
	paint := func(code, text string) string {
		if !color {
			return text
		}
		return code + text + colorReset
	}
	parts := []string{fmt.Sprintf("%dk", ro.Context/1000)}
	if !ro.Priced {
		return parts[0]
	}
	if ro.HasLastTurn {
		turn := fmt.Sprintf("turn $%.2f", ro.LastTurn)
		if ro.LastTurnHistory >= 0.005 {
			turn += fmt.Sprintf(" (hist $%.2f)", ro.LastTurnHistory)
		}
		parts = append(parts, turn)
	}
	if ro.Background >= 0.005 {
		parts = append(parts, fmt.Sprintf("bg +$%.2f", ro.Background))
	}
	switch {
	case ro.Cold:
		parts = append(parts, paint(colorCold, fmt.Sprintf("cold +$%.2f", ro.ColdNext)))
	case ro.ResetPays:
		parts = append(parts, paint(colorNudge, "↻ reset pays"))
	}
	return strings.Join(parts, " · ")
}
