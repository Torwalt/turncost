package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/Torwalt/turncost/internal/adapter"
	"github.com/Torwalt/turncost/internal/session"
	"github.com/Torwalt/turncost/internal/signal"
)

const usage = `turncost reads agent session transcripts and prices every turn.

Usage:
  turncost sessions [-root DIR] [-n N] [-since DURATION]
  turncost turns [-root DIR] <session-id-prefix | path.jsonl>
  turncost statusline [-- command args…]
  turncost handover write|take|drop|ls [-session ID] [-dir DIR]
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "statusline":
		runStatusline(os.Args[2:], os.Stdin, os.Stdout, os.Stderr, time.Now())
		return
	case "handover":
		os.Exit(runHandover(os.Args[2:], os.Stdin, os.Stdout, os.Stderr, time.Now()))
	case "sessions":
		err = runSessions(os.Args[2:], os.Stdout)
	case "turns":
		err = runTurns(os.Args[2:], os.Stdout)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "turncost:", err)
		os.Exit(1)
	}
}

func runSessions(args []string, w io.Writer) error {
	harness := adapter.Default()
	fs := flag.NewFlagSet("sessions", flag.ContinueOnError)
	root := fs.String("root", harness.DefaultRoot(), "Claude Code projects directory")
	n := fs.Int("n", 25, "sessions to list")
	since := fs.Duration("since", 0, "only sessions active within this duration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	paths, err := harness.Discover(*root)
	if err != nil {
		return err
	}
	sums := loadAll(harness, paths)
	if *since > 0 {
		cutoff := time.Now().Add(-*since)
		kept := sums[:0]
		for _, s := range sums {
			if t := lastActive(s.Session); t.After(cutoff) {
				kept = append(kept, s)
			}
		}
		sums = kept
	}
	sort.Slice(sums, func(i, j int) bool { return sums[i].Cost > sums[j].Cost })
	var total float64
	for _, s := range sums {
		total += s.Cost
	}
	fmt.Fprintf(w, "%d sessions, $%.2f at API list prices\n\n", len(sums), total)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "cost\tturns\treqs\tpeak\thit\tstart\tsession\tproject\t first prompt\t")
	for i, s := range sums {
		if i >= *n {
			break
		}
		fmt.Fprintf(tw, "$%.2f\t%d\t%d\t%s\t%.1f%%\t%s\t%s\t%s\t %s\t\n",
			s.Cost, prompts(s.Session), s.Requests, tokens(s.Peak), s.HitRate()*100,
			firstTime(s.Session).Format("01-02 15:04"), short(s.Session.ID, 8), lastPath(s.Session.Project),
			oneLine(s.Session.FirstPrompt(), 60))
	}
	return tw.Flush()
}

func runTurns(args []string, w io.Writer) error {
	harness := adapter.Default()
	fs := flag.NewFlagSet("turns", flag.ContinueOnError)
	root := fs.String("root", harness.DefaultRoot(), "Claude Code projects directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("turns takes one session id or path")
	}
	path, err := harness.Find(*root, fs.Arg(0))
	if err != nil {
		return err
	}
	s, err := harness.Load(path)
	if err != nil {
		return err
	}
	sum := signal.Analyze(s)
	fmt.Fprintf(w, "%s  %s  %s\n$%.2f over %d requests, baseline %s, peak %s, hit %.1f%%\n\n",
		s.ID, s.Project, s.Branch, sum.Cost, sum.Requests, tokens(sum.Baseline), tokens(sum.Peak), sum.HitRate()*100)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "#\ttime\tkind\treqs\tcontext\tcost\thistory\ttotal\tgap\t events | prompt\t")
	for _, t := range sum.Turns {
		if len(t.Turn.Requests) == 0 && t.Turn.Kind != session.TurnCommand {
			continue
		}
		gap := ""
		if t.Gap > 0 {
			gap = t.Gap.Round(time.Second).String()
			if t.Cold {
				gap += " cold"
			}
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%s\t$%.3f\t$%.3f\t$%.2f\t%s\t %s\t\n",
			t.Index, t.Turn.Time.Local().Format("15:04"), t.Turn.Kind, len(t.Turn.Requests), tokens(t.Context),
			t.Cost, t.HistoryCost, t.Cumulative, gap, describe(t.Turn))
	}
	return tw.Flush()
}

func loadAll(harness adapter.Adapter, paths []string) []signal.Summary {
	out := make([]signal.Summary, len(paths))
	ok := make([]bool, len(paths))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				s, err := harness.Load(paths[i])
				if err != nil || len(s.Requests()) == 0 {
					continue
				}
				out[i], ok[i] = signal.Analyze(s), true
			}
		}()
	}
	for i := range paths {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	var kept []signal.Summary
	for i := range out {
		if ok[i] {
			kept = append(kept, out[i])
		}
	}
	return kept
}

func describe(t session.Turn) string {
	counts := map[session.EventKind]int{}
	var order []string
	for _, e := range t.Events {
		if e.Kind == session.EventCommand {
			order = append(order, e.Detail)
			continue
		}
		counts[e.Kind]++
	}
	for _, k := range []session.EventKind{session.EventCommit, session.EventPush, session.EventPublish, session.EventEdit} {
		if counts[k] > 0 {
			order = append(order, fmt.Sprintf("%s×%d", k, counts[k]))
		}
	}
	text := strings.Join(order, " ")
	if t.Kind == session.TurnPrompt {
		if text != "" {
			text += " | "
		}
		text += oneLine(t.Prompt, 50)
	}
	return text
}

func prompts(s session.Session) int {
	n := 0
	for _, t := range s.Turns {
		if t.Kind == session.TurnPrompt {
			n++
		}
	}
	return n
}

func firstTime(s session.Session) time.Time {
	for _, t := range s.Turns {
		if !t.Time.IsZero() {
			return t.Time.Local()
		}
	}
	return time.Time{}
}

func lastActive(s session.Session) time.Time {
	var last time.Time
	for _, r := range s.Requests() {
		if r.Time.After(last) {
			last = r.Time
		}
	}
	return last
}

func tokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%dk", n/1_000)
	}
	return fmt.Sprint(n)
}

func short(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func lastPath(p string) string {
	if i := strings.LastIndex(p, "/repos/"); i >= 0 {
		return p[i+len("/repos/"):]
	}
	return p
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
