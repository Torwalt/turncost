package handover

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 11, 14, 2, 30, 0, time.UTC)

func newStore(t *testing.T) Store {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	return Default()
}

func TestRootFollowsXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/state")
	if got, want := DefaultRoot(), "/state/turncost/handovers"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/x")
	if got, want := DefaultRoot(), "/home/x/.local/state/turncost/handovers"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteThenTake(t *testing.T) {
	s := newStore(t)
	n, err := s.Write("-proj", "abcdef0123456789", "claude-code", "goal: ship\n\nnext: test\n", t0)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(s.Root, "-proj", "20261111T140230Z-abcdef01.md"); n.Path != want {
		t.Errorf("path %q, want %q", n.Path, want)
	}
	raw, err := os.ReadFile(n.Path)
	if err != nil {
		t.Fatal(err)
	}
	wantRaw := "from: abcdef0123456789\nharness: claude-code\nwritten: 2026-11-11T14:02:30Z\n---\ngoal: ship\n\nnext: test\n"
	if string(raw) != wantRaw {
		t.Errorf("file %q, want %q", raw, wantRaw)
	}

	w, ok, err := s.Waiting("-proj")
	if err != nil || !ok || w.Path != n.Path || w.State() != Waiting {
		t.Fatalf("waiting = %+v, %v, %v", w, ok, err)
	}
	taken, err := s.Take("-proj", "99999999-aaaa", t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if taken.Body != "goal: ship\n\nnext: test\n" {
		t.Errorf("body %q", taken.Body)
	}
	if taken.State() != Taken || taken.TakenBy != "99999999-aaaa" || !taken.TakenAt.Equal(t0.Add(time.Minute)) {
		t.Errorf("taken note = %+v", taken)
	}
	if _, ok, _ := s.Waiting("-proj"); ok {
		t.Error("a taken note still waits")
	}
	if _, err := s.Take("-proj", "99999999", t0); !errors.Is(err, ErrNoneWaiting) {
		t.Errorf("second take: %v, want ErrNoneWaiting", err)
	}
}

func TestTakeAndDropWithNothingWaiting(t *testing.T) {
	s := newStore(t)
	if _, err := s.Take("-proj", "s", t0); !errors.Is(err, ErrNoneWaiting) {
		t.Errorf("take: %v", err)
	}
	if _, err := s.Drop("-proj", t0); !errors.Is(err, ErrNoneWaiting) {
		t.Errorf("drop: %v", err)
	}
	if notes, err := s.List("-proj"); err != nil || len(notes) != 0 {
		t.Errorf("list = %v, %v", notes, err)
	}
}

func TestSecondWriteDropsTheFirst(t *testing.T) {
	s := newStore(t)
	first, _ := s.Write("-proj", "aaaaaaaa", "claude-code", "one", t0)
	second, err := s.Write("-proj", "bbbbbbbb", "claude-code", "two", t0.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	other, _ := s.Write("-other", "cccccccc", "claude-code", "elsewhere", t0)

	notes, err := s.List("-proj")
	if err != nil || len(notes) != 2 {
		t.Fatalf("list = %v, %v", notes, err)
	}
	if notes[0].Path != second.Path || notes[0].State() != Waiting {
		t.Errorf("newest = %+v", notes[0])
	}
	if notes[1].Path != first.Path || notes[1].State() != Dropped || !notes[1].DroppedAt.Equal(t0.Add(time.Hour)) {
		t.Errorf("older = %+v", notes[1])
	}
	if w, ok, _ := s.Waiting("-proj"); !ok || w.Body != "two\n" {
		t.Errorf("waiting = %+v", w)
	}
	if w, ok, _ := s.Waiting("-other"); !ok || w.Path != other.Path {
		t.Error("another project's note was touched")
	}
}

func TestDrop(t *testing.T) {
	s := newStore(t)
	s.Write("-proj", "aaaaaaaa", "claude-code", "one", t0)
	d, err := s.Drop("-proj", t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if d.State() != Dropped || !d.DroppedAt.Equal(t0.Add(time.Minute)) {
		t.Errorf("dropped = %+v", d)
	}
	if _, ok, _ := s.Waiting("-proj"); ok {
		t.Error("a dropped note still waits")
	}
}

func TestWritesInTheSameSecondKeepBoth(t *testing.T) {
	s := newStore(t)
	a, _ := s.Write("-proj", "aaaaaaaa", "claude-code", "one", t0)
	b, err := s.Write("-proj", "aaaaaaaa", "claude-code", "two", t0)
	if err != nil {
		t.Fatal(err)
	}
	if a.Path == b.Path {
		t.Fatal("the second write replaced the first")
	}
	if w, _, _ := s.Waiting("-proj"); w.Path != b.Path {
		t.Errorf("waiting = %s, want the second", w.Path)
	}
}

func TestBodyMayHoldTheSeparator(t *testing.T) {
	s := newStore(t)
	body := "intro\n---\nmore: stuff\n"
	s.Write("-proj", "aaaaaaaa", "claude-code", body, t0)
	w, _, _ := s.Waiting("-proj")
	if w.Body != body {
		t.Errorf("body %q, want %q", w.Body, body)
	}
}

func TestWriteRefusals(t *testing.T) {
	s := newStore(t)
	if _, err := s.Write("-proj", "a", "h", " \n", t0); err == nil {
		t.Error("an empty note was stored")
	}
	for _, p := range []string{"", "..", "a/b"} {
		if _, err := s.Write(p, "a", "h", "x", t0); err == nil || !strings.Contains(err.Error(), "invalid project") {
			t.Errorf("project %q: %v", p, err)
		}
	}
}

func TestSessionIDCannotEscapeTheDirectory(t *testing.T) {
	s := newStore(t)
	n, err := s.Write("-proj", "../../x\nfrom: evil", "h", "x", t0)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(n.Path) != filepath.Join(s.Root, "-proj") {
		t.Errorf("note landed at %s", n.Path)
	}
	w, _, _ := s.Waiting("-proj")
	if w.From != "../../x from: evil" {
		t.Errorf("from = %q", w.From)
	}
}
