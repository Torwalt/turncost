package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Torwalt/turncost/internal/adapter"
	"github.com/Torwalt/turncost/internal/handover"
)

// TestMain keeps every test off the real handover store and transcripts.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "turncost-test")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	os.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(dir, "claude"))
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type project struct {
	dir     string // working directory of the project
	session string
}

// newProject isolates the state and transcripts of one test and gives it a
// working directory with a current session whose transcript holds only a
// /clear.
func newProject(t *testing.T, session string) project {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	p := project{dir: filepath.Join(t.TempDir(), "repo"), session: session}
	if err := os.MkdirAll(p.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	transcriptDir := filepath.Join(adapter.Default().DefaultRoot(), adapter.Default().ProjectKey(p.dir))
	if err := os.MkdirAll(transcriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	empty, err := os.ReadFile("testdata/empty.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(transcriptDir, session+".jsonl"), empty, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func handoverCmd(t *testing.T, stdin string, now time.Time, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errw bytes.Buffer
	code = runHandover(args, strings.NewReader(stdin), &out, &errw, now)
	return out.String(), errw.String(), code
}

var noteTime = time.Date(2026, 11, 11, 14, 2, 0, 0, time.UTC)

func TestHandoverWriteThenTake(t *testing.T) {
	p := newProject(t, "aaaaaaaa-1111")
	out, errw, code := handoverCmd(t, "goal: x\n", noteTime, "write", "-dir", p.dir)
	if code != 0 || errw != "" {
		t.Fatalf("write: code %d, stderr %q", code, errw)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 || lines[1] != "next: /clear, then /takeover" {
		t.Fatalf("write printed %q", out)
	}
	if !strings.HasSuffix(lines[0], "20261111T140200Z-aaaaaaaa.md") {
		t.Errorf("path %q", lines[0])
	}
	if _, err := os.Stat(lines[0]); err != nil {
		t.Errorf("printed path is not a file: %v", err)
	}

	// The /clear starts a new session in the same directory.
	newTranscript(t, p, "bbbbbbbb-2222")
	out, errw, code = handoverCmd(t, "", noteTime.Add(time.Hour), "take", "-dir", p.dir)
	if code != 0 || errw != "" || out != "goal: x\n" {
		t.Fatalf("take: code %d, stdout %q, stderr %q", code, out, errw)
	}
	notes, _ := handover.Default().List(adapter.Default().ProjectKey(p.dir))
	if len(notes) != 1 || notes[0].State() != handover.Taken || notes[0].TakenBy != "bbbbbbbb-2222" {
		t.Errorf("notes = %+v", notes)
	}
}

// newTranscript adds a newer session to the project's transcripts.
func newTranscript(t *testing.T, p project, id string) string {
	t.Helper()
	dir := filepath.Join(adapter.Default().DefaultRoot(), adapter.Default().ProjectKey(p.dir))
	path := filepath.Join(dir, id+".jsonl")
	data, err := os.ReadFile(filepath.Join(dir, p.session+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHandoverTakeWithNothingWaiting(t *testing.T) {
	p := newProject(t, "aaaaaaaa-1111")
	out, errw, code := handoverCmd(t, "", noteTime, "take", "-dir", p.dir)
	if code != 1 || out != "" {
		t.Errorf("code %d, stdout %q", code, out)
	}
	if want := "no handover waiting for " + p.dir + "\n"; errw != want {
		t.Errorf("stderr %q, want %q", errw, want)
	}
	// Even with no session to name.
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	if _, _, code := handoverCmd(t, "", noteTime, "take", "-dir", p.dir); code != 1 {
		t.Errorf("code %d without a transcript", code)
	}
}

func TestHandoverSecondWriteDropsTheFirst(t *testing.T) {
	p := newProject(t, "aaaaaaaa-1111")
	handoverCmd(t, "first\n", noteTime, "write", "-dir", p.dir)
	if _, errw, code := handoverCmd(t, "second\n", noteTime.Add(time.Hour), "write", "-dir", p.dir, "-session", "cccccccc-3"); code != 0 {
		t.Fatalf("write: %d %s", code, errw)
	}
	out, _, _ := handoverCmd(t, "", noteTime, "ls", "-dir", p.dir)
	rows := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(rows) != 2 || !strings.HasPrefix(rows[0], "waiting") || !strings.Contains(rows[0], "second") ||
		!strings.HasPrefix(rows[1], "dropped") || !strings.Contains(rows[1], "first") {
		t.Errorf("ls =\n%s", out)
	}
	out, _, code := handoverCmd(t, "", noteTime, "take", "-dir", p.dir, "-session", "dddddddd")
	if code != 0 || out != "second\n" {
		t.Errorf("take = %q, code %d", out, code)
	}
}

func TestHandoverDrop(t *testing.T) {
	p := newProject(t, "aaaaaaaa-1111")
	handoverCmd(t, "note\n", noteTime, "write", "-dir", p.dir)
	if _, errw, code := handoverCmd(t, "", noteTime, "drop", "-dir", p.dir); code != 0 {
		t.Fatalf("drop: %d %s", code, errw)
	}
	if _, errw, code := handoverCmd(t, "", noteTime, "take", "-dir", p.dir); code != 1 || !strings.HasPrefix(errw, "no handover waiting") {
		t.Errorf("take after drop: %d %q", code, errw)
	}
	if _, errw, code := handoverCmd(t, "", noteTime, "drop", "-dir", p.dir); code != 1 || !strings.HasPrefix(errw, "no handover waiting") {
		t.Errorf("second drop: %d %q", code, errw)
	}
}

func TestHandoverLsWithNoNotes(t *testing.T) {
	p := newProject(t, "aaaaaaaa-1111")
	out, _, code := handoverCmd(t, "", noteTime, "ls", "-dir", p.dir)
	if code != 0 || out != "no handovers for "+p.dir+"\n" {
		t.Errorf("code %d, out %q", code, out)
	}
}

func TestHandoverMisuse(t *testing.T) {
	newProject(t, "aaaaaaaa-1111")
	for _, args := range [][]string{nil, {"nope"}, {"ls", "extra"}, {"drop", "-session", "x"}} {
		if _, _, code := handoverCmd(t, "", noteTime, args...); code != 2 {
			t.Errorf("%v: code %d, want 2", args, code)
		}
	}
	if _, errw, code := handoverCmd(t, "  \n", noteTime, "write", "-session", "x"); code != 1 || !strings.Contains(errw, "empty") {
		t.Errorf("empty note: code %d, %q", code, errw)
	}
}

func TestStatuslineShowsAWaitingHandover(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	written := time.Date(2026, 11, 11, 14, 2, 0, 0, time.UTC)
	key := adapter.Default().ProjectKey("/repo")
	if _, err := handover.Default().Write(key, "aaaaaaaa", "claude-code", "note", written); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("handover waiting since %s · /takeover\n", written.Local().Format("15:04"))

	clearOnly := payload(t, "testdata/empty.jsonl")
	if got := statusline(t, clearOnly, warmNow); got != want {
		t.Errorf("only a /clear: got %q, want %q", got, want)
	}
	if got := statusline(t, payload(t, "testdata/absent.jsonl"), warmNow); got != want {
		t.Errorf("no transcript yet: got %q, want %q", got, want)
	}
	if got := statusline(t, `{"cwd":"/repo"}`, warmNow); got != want {
		t.Errorf("no transcript path: got %q, want %q", got, want)
	}
	// Joined to the user's own statusline like any segment.
	if got := statusline(t, clearOnly, warmNow, "--", script(t, `printf 'main *'`)); got != "main * · "+want {
		t.Errorf("wrapped: got %q", got)
	}

	// A session that has made a request shows its readout instead.
	if got := statusline(t, payload(t, "testdata/warm.jsonl"), warmNow); !strings.HasPrefix(got, "450k · ") {
		t.Errorf("session with requests: got %q", got)
	}
	// Another project, a taken note and a dropped note show nothing.
	if got := statusline(t, `{"cwd":"/elsewhere"}`, warmNow); got != "" {
		t.Errorf("other project: got %q", got)
	}
	if _, err := handover.Default().Take(key, "bbbbbbbb", warmNow); err != nil {
		t.Fatal(err)
	}
	if got := statusline(t, clearOnly, warmNow); got != "" {
		t.Errorf("after take: got %q", got)
	}
}
