package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Torwalt/turncost/internal/adapter"
	"github.com/Torwalt/turncost/internal/handover"
)

const handoverUsage = `Usage:
  turncost handover write [-session ID] [-dir DIR]   note on stdin
  turncost handover take  [-session ID] [-dir DIR]
  turncost handover drop  [-dir DIR]
  turncost handover ls    [-dir DIR]
`

// runHandover runs a `turncost handover` subcommand and returns the exit
// status: 0 done, 1 failed, 2 misused.
func runHandover(args []string, stdin io.Reader, w, errw io.Writer, now time.Time) int {
	if len(args) == 0 {
		fmt.Fprint(errw, handoverUsage)
		return 2
	}
	harness := adapter.Default()
	store := handover.Default()
	fs := flag.NewFlagSet("handover "+args[0], flag.ContinueOnError)
	fs.SetOutput(errw)
	dir := fs.String("dir", "", "project working directory (default: the working directory)")
	var session *string
	switch args[0] {
	case "write", "take":
		session = fs.String("session", "", "session id (default: the current session of the directory)")
	case "drop", "ls":
	default:
		fmt.Fprint(errw, handoverUsage)
		return 2
	}
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprint(errw, handoverUsage)
		return 2
	}
	project, err := projectDir(*dir)
	if err != nil {
		fmt.Fprintln(errw, "turncost:", err)
		return 1
	}
	key := harness.ProjectKey(project)
	fail := func(err error) int {
		fmt.Fprintln(errw, "turncost:", err)
		return 1
	}
	none := func() int {
		fmt.Fprintf(errw, "no handover waiting for %s\n", project)
		return 1
	}
	// currentSession is the session id to record: -session, else the adapter's
	// current session of the directory.
	currentSession := func() (string, error) {
		if *session != "" {
			return *session, nil
		}
		path, err := harness.Current(harness.DefaultRoot(), project)
		if err != nil {
			return "", fmt.Errorf("%w; pass -session", err)
		}
		s, err := harness.Load(path)
		if err != nil {
			return "", err
		}
		return s.ID, nil
	}

	switch args[0] {
	case "write":
		body, err := io.ReadAll(stdin)
		if err != nil {
			return fail(err)
		}
		id, err := currentSession()
		if err != nil {
			return fail(err)
		}
		n, err := store.Write(key, id, harness.Name(), string(body), now)
		if err != nil {
			return fail(err)
		}
		fmt.Fprintln(w, n.Path)
		fmt.Fprintln(w, "next: /clear, then /takeover")
	case "take":
		if _, ok, err := store.Waiting(key); err != nil {
			return fail(err)
		} else if !ok {
			return none()
		}
		id, err := currentSession()
		if err != nil {
			return fail(err)
		}
		n, err := store.Take(key, id, now)
		if errors.Is(err, handover.ErrNoneWaiting) {
			return none()
		}
		if err != nil {
			return fail(err)
		}
		fmt.Fprint(w, n.Body)
	case "drop":
		_, err := store.Drop(key, now)
		if errors.Is(err, handover.ErrNoneWaiting) {
			return none()
		}
		if err != nil {
			return fail(err)
		}
	case "ls":
		notes, err := store.List(key)
		if err != nil {
			return fail(err)
		}
		if len(notes) == 0 {
			fmt.Fprintf(w, "no handovers for %s\n", project)
			return 0
		}
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		for _, n := range notes {
			fmt.Fprintf(tw, "%s\t%s\tfrom %s\t%s\t%s\n", n.State(), n.Written.Local().Format("2006-01-02 15:04"),
				short(n.From, 8), settledText(n), oneLine(firstLine(n.Body), 60))
		}
		return flushed(tw, errw)
	}
	return 0
}

func flushed(tw *tabwriter.Writer, errw io.Writer) int {
	if err := tw.Flush(); err != nil {
		fmt.Fprintln(errw, "turncost:", err)
		return 1
	}
	return 0
}

// projectDir is the absolute form of -dir, the working directory by default.
func projectDir(dir string) (string, error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return wd, nil
	}
	return filepath.Abs(dir)
}

// settledText says who took a note or when it was dropped.
func settledText(n handover.Note) string {
	switch n.State() {
	case handover.Taken:
		return fmt.Sprintf("by %s %s", short(n.TakenBy, 8), n.TakenAt.Local().Format("2006-01-02 15:04"))
	case handover.Dropped:
		return n.DroppedAt.Local().Format("2006-01-02 15:04")
	}
	return ""
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
