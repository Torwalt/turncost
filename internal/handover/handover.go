// Package handover stores the notes a session leaves for the next one.
//
// A note is a file under a project's directory: `key: value` header lines, a
// line `---`, then the body. The store knows no harness format; a project is
// an opaque key the caller derives from a working directory.
package handover

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	fileTime  = "20060102T150405Z"
	separator = "---"
)

// ErrNoneWaiting is returned when a project has no waiting note.
var ErrNoneWaiting = errors.New("no handover waiting")

// State of a note.
type State string

const (
	Waiting State = "waiting"
	Taken   State = "taken"
	Dropped State = "dropped"
)

// Note is one stored handover.
type Note struct {
	Path    string
	From    string // session that wrote it
	Harness string
	Written time.Time

	TakenBy string // session that took it
	TakenAt time.Time
	// DroppedAt is set when the note was dropped.
	DroppedAt time.Time

	Body string

	header []string // key: value lines as stored, in order
}

// State reports whether the note waits, was taken or was dropped.
func (n Note) State() State {
	switch {
	case n.TakenBy != "" || !n.TakenAt.IsZero():
		return Taken
	case !n.DroppedAt.IsZero():
		return Dropped
	}
	return Waiting
}

// Store is a directory of notes, one subdirectory per project.
type Store struct {
	Root string
}

// DefaultRoot is `$XDG_STATE_HOME/turncost/handovers`, by default
// `~/.local/state/turncost/handovers`.
func DefaultRoot() string {
	return filepath.Join(StateDir(), "handovers")
}

// StateDir is `$XDG_STATE_HOME/turncost`, by default
// `~/.local/state/turncost`.
func StateDir() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "turncost")
}

// Default is the store under DefaultRoot.
func Default() Store { return Store{Root: DefaultRoot()} }

func (s Store) dir(project string) (string, error) {
	if project == "" || project != filepath.Base(project) || project == "." || project == ".." {
		return "", fmt.Errorf("invalid project %q", project)
	}
	return filepath.Join(s.Root, project), nil
}

// Write stores body as the newest note of project and marks the project's
// older waiting notes dropped, so at most one waits.
func (s Store) Write(project, from, harness, body string, now time.Time) (Note, error) {
	dir, err := s.dir(project)
	if err != nil {
		return Note{}, err
	}
	if strings.TrimSpace(body) == "" {
		return Note{}, errors.New("empty handover note")
	}
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Note{}, err
	}
	now = now.UTC().Truncate(time.Second)
	id := fileSafe(from)
	for {
		path := filepath.Join(dir, now.Format(fileTime)+"-"+short(id, 8)+".md")
		n := Note{Path: path, From: oneLine(from), Harness: oneLine(harness), Written: now, Body: body}
		n.header = []string{
			"from: " + n.From,
			"harness: " + n.Harness,
			"written: " + now.Format(time.RFC3339),
		}
		err := create(path, n.render())
		if errors.Is(err, fs.ErrExist) {
			now = now.Add(time.Second)
			continue
		}
		if err != nil {
			return Note{}, err
		}
		older, err := s.List(project)
		if err != nil {
			return n, err
		}
		for _, o := range older {
			if o.Path != n.Path && o.State() == Waiting {
				if err := settle(o, "dropped: "+now.Format(time.RFC3339)); err != nil {
					return n, err
				}
			}
		}
		return n, nil
	}
}

// List returns the notes of project, newest first. A project without notes
// has none.
func (s Store) List(project string) ([]Note, error) {
	dir, err := s.dir(project)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var notes []Note
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		n, err := read(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // not a note of ours
		}
		notes = append(notes, n)
	}
	sort.SliceStable(notes, func(i, j int) bool {
		if !notes[i].Written.Equal(notes[j].Written) {
			return notes[i].Written.After(notes[j].Written)
		}
		return notes[i].Path > notes[j].Path
	})
	return notes, nil
}

// Waiting returns the newest note of project that is neither taken nor
// dropped.
func (s Store) Waiting(project string) (Note, bool, error) {
	notes, err := s.List(project)
	if err != nil {
		return Note{}, false, err
	}
	for _, n := range notes {
		if n.State() == Waiting {
			return n, true, nil
		}
	}
	return Note{}, false, nil
}

// Take marks the waiting note of project taken by session and returns it.
func (s Store) Take(project, session string, now time.Time) (Note, error) {
	n, ok, err := s.Waiting(project)
	if err != nil {
		return Note{}, err
	}
	if !ok {
		return Note{}, ErrNoneWaiting
	}
	line := "taken: " + oneLine(session) + " " + now.UTC().Format(time.RFC3339)
	if err := settle(n, line); err != nil {
		return Note{}, err
	}
	return read(n.Path)
}

// Drop marks the waiting note of project dropped and returns it.
func (s Store) Drop(project string, now time.Time) (Note, error) {
	n, ok, err := s.Waiting(project)
	if err != nil {
		return Note{}, err
	}
	if !ok {
		return Note{}, ErrNoneWaiting
	}
	if err := settle(n, "dropped: "+now.UTC().Format(time.RFC3339)); err != nil {
		return Note{}, err
	}
	return read(n.Path)
}

// settle appends a header line to the note's file.
func settle(n Note, line string) error {
	n.header = append(n.header, line)
	return replace(n.Path, n.render())
}

func (n Note) render() string {
	return strings.Join(n.header, "\n") + "\n" + separator + "\n" + n.Body
}

func read(path string) (Note, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Note{}, err
	}
	head, body, ok := strings.Cut(string(data), "\n"+separator+"\n")
	if !ok {
		return Note{}, errors.New("no header in " + path)
	}
	n := Note{Path: path, Body: body}
	for l := range strings.SplitSeq(head, "\n") {
		n.header = append(n.header, l)
		key, value, _ := strings.Cut(l, ":")
		value = strings.TrimSpace(value)
		switch key {
		case "from":
			n.From = value
		case "harness":
			n.Harness = value
		case "written":
			n.Written, err = time.Parse(time.RFC3339, value)
		case "taken":
			who, at, _ := strings.Cut(value, " ")
			n.TakenBy = who
			n.TakenAt, err = time.Parse(time.RFC3339, at)
		case "dropped":
			n.DroppedAt, err = time.Parse(time.RFC3339, value)
		}
		if err != nil {
			return Note{}, fmt.Errorf("%s: %s: %w", path, key, err)
		}
	}
	if n.Written.IsZero() {
		return Note{}, errors.New("no written time in " + path)
	}
	return n, nil
}

// create writes a new file, failing with fs.ErrExist if it is there.
func create(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	return f.Close()
}

// replace swaps the file's content in one step.
func replace(path, content string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".note-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// oneLine keeps a header value on one line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// fileSafe keeps only characters fit for a file name.
func fileSafe(s string) string {
	id := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return -1
	}, s)
	if id == "" {
		return "unknown"
	}
	return id
}

func short(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
