package session

import "time"

type Usage struct {
	Input        int64
	CacheRead    int64
	CacheWrite5m int64
	CacheWrite1h int64
	Output       int64
}

func (u Usage) Prompt() int64 {
	return u.Input + u.CacheRead + u.CacheWrite5m + u.CacheWrite1h
}

func (u Usage) Add(o Usage) Usage {
	return Usage{
		Input:        u.Input + o.Input,
		CacheRead:    u.CacheRead + o.CacheRead,
		CacheWrite5m: u.CacheWrite5m + o.CacheWrite5m,
		CacheWrite1h: u.CacheWrite1h + o.CacheWrite1h,
		Output:       u.Output + o.Output,
	}
}

type Request struct {
	ID        string
	Time      time.Time
	Model     string
	Usage     Usage
	Sidechain bool
}

type TurnKind string

const (
	TurnPrompt       TurnKind = "prompt"
	TurnCommand      TurnKind = "command"
	TurnNotification TurnKind = "notification"
	TurnContinuation TurnKind = "continuation"
)

type EventKind string

const (
	EventCommit  EventKind = "commit"
	EventPush    EventKind = "push"
	EventPublish EventKind = "publish"
	EventEdit    EventKind = "edit"
	EventCommand EventKind = "command"
)

type Event struct {
	Kind   EventKind
	Detail string
}

type Turn struct {
	Time     time.Time
	Kind     TurnKind
	Prompt   string
	Requests []Request
	Events   []Event
}

type Session struct {
	ID      string
	Harness string
	Path    string
	Project string
	Branch  string
	Version string
	Turns   []Turn
}

func (s Session) Requests() []Request {
	var out []Request
	for _, t := range s.Turns {
		out = append(out, t.Requests...)
	}
	return out
}

func (s Session) FirstPrompt() string {
	for _, t := range s.Turns {
		if t.Kind == TurnPrompt {
			return t.Prompt
		}
	}
	return ""
}

// Ref points at a session a harness is running: where its transcript is, its
// id and its working directory. Any field may be empty.
type Ref struct {
	ID         string
	Transcript string
	Dir        string
}
