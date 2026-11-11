package claudecode

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Torwalt/turncost/internal/session"
)

const Harness = "claude-code"

// Adapter implements adapter.Adapter for Claude Code.
type Adapter struct{}

func (Adapter) Name() string                              { return Harness }
func (Adapter) DefaultRoot() string                       { return DefaultRoot() }
func (Adapter) Discover(root string) ([]string, error)    { return Discover(root) }
func (Adapter) Find(root, id string) (string, error)      { return Find(root, id) }
func (Adapter) Load(path string) (session.Session, error) { return Load(path) }
func (Adapter) Current(root, dir string) (string, error)  { return Current(root, dir) }
func (Adapter) ProjectKey(dir string) string              { return ProjectKey(dir) }
func (Adapter) ProjectDir(root, dir string) string        { return ProjectDir(root, dir) }
func (Adapter) ParseStatusline(data []byte) (session.Ref, error) {
	return ParseStatusline(data)
}
func (Adapter) ParsePrompt(data []byte) (session.Ref, string, error) {
	return ParsePrompt(data)
}

func DefaultRoot() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "projects")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "projects")
}

func Discover(root string) ([]string, error) {
	var paths []string
	projects, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		matches, err := filepath.Glob(filepath.Join(root, p.Name(), "*.jsonl"))
		if err != nil {
			return nil, err
		}
		paths = append(paths, matches...)
	}
	sort.Strings(paths)
	return paths, nil
}

func Find(root, id string) (string, error) {
	if strings.HasSuffix(id, ".jsonl") {
		return id, nil
	}
	matches, err := filepath.Glob(filepath.Join(root, "*", id+"*.jsonl"))
	if err != nil {
		return "", err
	}
	switch len(matches) {
	case 0:
		return "", errors.New("no session matches " + id)
	case 1:
		return matches[0], nil
	default:
		return "", errors.New("several sessions match " + id)
	}
}

// EncodeDir returns the name of the project directory Claude Code uses for the
// working directory dir: every character other than an ASCII letter or digit
// becomes '-'.
func EncodeDir(dir string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, dir)
}

// ProjectDir returns the directory whose sessions dir belongs to: dir or its
// nearest ancestor that has a project directory under root, else dir itself.
// An agent's shell can move into a subdirectory of the one its session was
// started in; the session still belongs to the starting one.
func ProjectDir(root, dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	for d := abs; ; d = filepath.Dir(d) {
		if info, err := os.Stat(filepath.Join(root, EncodeDir(d))); err == nil && info.IsDir() {
			return d
		}
		if parent := filepath.Dir(d); parent == d {
			return abs
		}
	}
}

// ProjectKey names the project of the working directory dir: its project
// directory's name.
func ProjectKey(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return EncodeDir(dir)
}

// Current returns the transcript of the most recently modified session in the
// project directory of dir. Two sessions open in one directory at once can
// make it pick the wrong one.
func Current(root, dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	project := filepath.Join(root, EncodeDir(abs))
	matches, err := filepath.Glob(filepath.Join(project, "*.jsonl"))
	if err != nil {
		return "", err
	}
	var best string
	var bestTime time.Time
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil || info.IsDir() {
			continue
		}
		if best == "" || info.ModTime().After(bestTime) {
			best, bestTime = m, info.ModTime()
		}
	}
	if best == "" {
		return "", errors.New("no session transcript in " + project)
	}
	return best, nil
}

// ParseStatusline reads the JSON Claude Code pipes to a statusline command.
// Only the session id, transcript path, directory and the share of the
// context window left (context_window.remaining_percentage) are used: the directory
// directory is the one the session was started in (workspace.project_dir),
// else the working directory, which follows the agent's shell into
// subdirectories.
func ParseStatusline(data []byte) (session.Ref, error) {
	var in struct {
		SessionID      string `json:"session_id"`
		TranscriptPath string `json:"transcript_path"`
		Cwd            string `json:"cwd"`
		Workspace      struct {
			ProjectDir string `json:"project_dir"`
		} `json:"workspace"`
		ContextWindow struct {
			RemainingPercentage *float64 `json:"remaining_percentage"`
		} `json:"context_window"`
	}
	if err := json.Unmarshal(data, &in); err != nil {
		return session.Ref{}, err
	}
	dir := in.Workspace.ProjectDir
	if dir == "" {
		dir = in.Cwd
	}
	ref := session.Ref{ID: in.SessionID, Transcript: in.TranscriptPath, Dir: dir}
	if left := in.ContextWindow.RemainingPercentage; left != nil {
		ref.ContextLeft, ref.HasContextLeft = *left, true
	}
	return ref, nil
}

// ParsePrompt reads the JSON a UserPromptSubmit hook gets on stdin: the
// session it runs in and the prompt as typed (a slash command arrives
// unexpanded).
func ParsePrompt(data []byte) (session.Ref, string, error) {
	var in struct {
		SessionID      string `json:"session_id"`
		TranscriptPath string `json:"transcript_path"`
		Cwd            string `json:"cwd"`
		Prompt         string `json:"prompt"`
	}
	if err := json.Unmarshal(data, &in); err != nil {
		return session.Ref{}, "", err
	}
	return session.Ref{ID: in.SessionID, Transcript: in.TranscriptPath, Dir: in.Cwd}, in.Prompt, nil
}

func Load(path string) (session.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return session.Session{}, err
	}
	defer f.Close()
	s := session.Session{Harness: Harness, Path: path, ID: strings.TrimSuffix(filepath.Base(path), ".jsonl")}
	p := parser{seen: map[string]bool{}}
	if err := p.read(f, &s, false); err != nil {
		return s, err
	}
	subagents, _ := filepath.Glob(filepath.Join(strings.TrimSuffix(path, ".jsonl"), "subagents", "*.jsonl"))
	for _, sub := range subagents {
		if err := p.attach(sub, &s); err != nil {
			return s, err
		}
	}
	return s, nil
}

type line struct {
	Type        string    `json:"type"`
	SessionID   string    `json:"sessionId"`
	RequestID   string    `json:"requestId"`
	Timestamp   time.Time `json:"timestamp"`
	Cwd         string    `json:"cwd"`
	GitBranch   string    `json:"gitBranch"`
	Version     string    `json:"version"`
	IsMeta      bool      `json:"isMeta"`
	IsSidechain bool      `json:"isSidechain"`
	Message     struct {
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
		Usage   *struct {
			InputTokens              int64 `json:"input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheCreation            *struct {
				FiveMinute int64 `json:"ephemeral_5m_input_tokens"`
				OneHour    int64 `json:"ephemeral_1h_input_tokens"`
			} `json:"cache_creation"`
		} `json:"usage"`
	} `json:"message"`
}

type block struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Name  string `json:"name"`
	Input struct {
		Command  string `json:"command"`
		FilePath string `json:"file_path"`
	} `json:"input"`
}

type parser struct {
	seen map[string]bool
}

func (p *parser) read(r io.Reader, s *session.Session, sidechain bool) error {
	br := bufio.NewReaderSize(r, 1<<20)
	for {
		raw, err := br.ReadBytes('\n')
		if len(raw) > 0 {
			var l line
			if json.Unmarshal(raw, &l) == nil {
				p.apply(l, s, sidechain)
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (p *parser) attach(path string, s *session.Session) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var sub session.Session
	if err := p.read(f, &sub, true); err != nil {
		return err
	}
	for _, r := range sub.Requests() {
		i := turnAt(s.Turns, r.Time)
		if i < 0 {
			continue
		}
		s.Turns[i].Requests = append(s.Turns[i].Requests, r)
	}
	return nil
}

func turnAt(turns []session.Turn, t time.Time) int {
	i := sort.Search(len(turns), func(i int) bool { return turns[i].Time.After(t) })
	return i - 1
}

func (p *parser) apply(l line, s *session.Session, sidechain bool) {
	if s.Project == "" && l.Cwd != "" {
		s.Project, s.Branch, s.Version = l.Cwd, l.GitBranch, l.Version
	}
	switch l.Type {
	case "user":
		if l.IsMeta {
			return
		}
		text, ok := userText(l.Message.Content)
		if !ok {
			return
		}
		kind, events, keep := classify(text)
		if !keep {
			return
		}
		s.Turns = append(s.Turns, session.Turn{Time: l.Timestamp, Kind: kind, Prompt: text, Events: events})
	case "assistant":
		if len(s.Turns) == 0 {
			s.Turns = append(s.Turns, session.Turn{Time: l.Timestamp, Kind: session.TurnContinuation})
		}
		t := &s.Turns[len(s.Turns)-1]
		t.Events = append(t.Events, toolEvents(l.Message.Content)...)
		u := l.Message.Usage
		key := l.RequestID
		if key == "" {
			key = l.Message.ID
		}
		if u == nil || key == "" || p.seen[key] {
			return
		}
		p.seen[key] = true
		usage := session.Usage{Input: u.InputTokens, CacheRead: u.CacheReadInputTokens, Output: u.OutputTokens}
		if c := u.CacheCreation; c != nil {
			usage.CacheWrite5m, usage.CacheWrite1h = c.FiveMinute, c.OneHour
		}
		if rest := u.CacheCreationInputTokens - usage.CacheWrite5m - usage.CacheWrite1h; rest > 0 {
			usage.CacheWrite5m += rest
		}
		t.Requests = append(t.Requests, session.Request{
			ID: key, Time: l.Timestamp, Model: l.Message.Model, Usage: usage,
			Sidechain: sidechain || l.IsSidechain,
		})
	}
}

func userText(raw json.RawMessage) (string, bool) {
	var str string
	if json.Unmarshal(raw, &str) == nil {
		return str, strings.TrimSpace(str) != ""
	}
	var blocks []block
	if json.Unmarshal(raw, &blocks) != nil {
		return "", false
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n"), len(parts) > 0
}

var commandName = regexp.MustCompile(`<command-name>([^<]+)</command-name>`)

func classify(text string) (session.TurnKind, []session.Event, bool) {
	trimmed := strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(trimmed, "<local-command-"):
		return "", nil, false
	case strings.HasPrefix(trimmed, "<command-"):
		name := "?"
		if m := commandName.FindStringSubmatch(trimmed); m != nil {
			name = m[1]
		}
		return session.TurnCommand, []session.Event{{Kind: session.EventCommand, Detail: name}}, true
	case strings.HasPrefix(trimmed, "<task-notification>"):
		return session.TurnNotification, nil, true
	case strings.HasPrefix(trimmed, "This session is being continued"):
		return session.TurnContinuation, nil, true
	}
	return session.TurnPrompt, nil, true
}

func toolEvents(raw json.RawMessage) []session.Event {
	var blocks []block
	if json.Unmarshal(raw, &blocks) != nil {
		return nil
	}
	var out []session.Event
	for _, b := range blocks {
		if b.Type != "tool_use" {
			continue
		}
		switch b.Name {
		case "Bash":
			if strings.Contains(b.Input.Command, "git commit") {
				out = append(out, session.Event{Kind: session.EventCommit})
			}
			if strings.Contains(b.Input.Command, "git push") {
				out = append(out, session.Event{Kind: session.EventPush})
			}
		case "Artifact":
			out = append(out, session.Event{Kind: session.EventPublish})
		case "Edit", "Write", "NotebookEdit":
			out = append(out, session.Event{Kind: session.EventEdit, Detail: b.Input.FilePath})
		}
	}
	return out
}
