// Package adapter is the harness-neutral surface the CLI reaches coding-agent
// harnesses through. Each harness lives in its own subpackage and is the only
// code that knows that harness's files.
package adapter

import (
	"github.com/Torwalt/turncost/internal/adapter/claudecode"
	"github.com/Torwalt/turncost/internal/session"
)

// Adapter turns one harness's transcripts into sessions.
type Adapter interface {
	// Name is the harness name recorded in Session.Harness.
	Name() string
	// DefaultRoot is where the harness keeps its transcripts.
	DefaultRoot() string
	// Discover lists the transcripts under root.
	Discover(root string) ([]string, error)
	// Find resolves a session id, id prefix or transcript path to a transcript.
	Find(root, id string) (string, error)
	// Load parses one transcript.
	Load(path string) (session.Session, error)
	// Current returns the transcript of the current session of the working
	// directory dir: the most recently modified one of its project.
	Current(root, dir string) (string, error)
}

// Default returns the adapter the CLI uses. Claude Code is the only harness so
// far; this is the one place that selects it.
func Default() Adapter {
	return claudecode.Adapter{}
}
