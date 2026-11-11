package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Torwalt/turncost/internal/adapter"
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

// statuslineSegment is the segment for the session the statusline input names;
// empty when the session has no main request yet or its transcript is not
// there yet.
func statuslineSegment(input []byte, now time.Time) (string, error) {
	harness := adapter.Default()
	ref, err := harness.ParseStatusline(input)
	if err != nil {
		return "", err
	}
	if ref.Transcript == "" {
		return "", nil
	}
	s, err := harness.Load(ref.Transcript)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return segment(signal.Read(s, now, pricing.Lookup)), nil
}

// segment formats a readout as the statusline text.
func segment(ro signal.Readout) string {
	if !ro.HasMain {
		return ""
	}
	parts := []string{fmt.Sprintf("%dk", ro.Context/1000)}
	if !ro.Priced {
		return parts[0]
	}
	lastTurn := func() {
		if ro.HasLastTurn {
			parts = append(parts, fmt.Sprintf("last turn $%.2f", ro.LastTurn))
		}
	}
	if ro.Cold {
		parts = append(parts, fmt.Sprintf("cold: next step $%.2f", ro.ColdNext))
		lastTurn()
		return strings.Join(parts, " · ")
	}
	lastTurn()
	if !ro.HasPayback {
		parts = append(parts, fmt.Sprintf("step $%.3f", ro.Step))
		return strings.Join(parts, " · ")
	}
	parts = append(parts, fmt.Sprintf("step $%.3f (%.0f%% history)", ro.Step, ro.HistoryShare*100))
	pays := fmt.Sprintf("reset pays in ~%d %s", ro.PaybackSteps, plural(ro.PaybackSteps, "step"))
	if ro.PaybackTurns > 0 {
		pays += " (" + turnsText(ro.PaybackTurns) + ")"
	}
	return strings.Join(append(parts, pays), " · ")
}

func turnsText(turns float64) string {
	if turns < 1 {
		return "<1 turn"
	}
	n := int(math.Round(turns))
	return fmt.Sprintf("~%d %s", n, plural(n, "turn"))
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
