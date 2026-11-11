---
kind: change
area: watcher
state: open
---

# Optional findings from the STATUSLINE_HANDOVER review

*Review, 2026-11-11.* Collected by the direct review of
`docs/plans/STATUSLINE_HANDOVER.md` (review pass
`docs/plans/STATUSLINE_HANDOVER_REVIEW.md`) at revision
`fcea995649393d152168b985543b766fa0569b5b`, base
`d2dbc807d1e9ebf5c1e6f7c68ac61291ab593257`. The work meets the plan without
these; they are left for an interactive session to keep, split or discard.

**Now:** the handover CLI keys a note by the working directory it runs in
(`cmd/turncost/handover.go`, `runHandover`: `projectDir` then
`harness.ProjectKey`), and `-session` defaults to `harness.Current(root, dir)`
(`internal/adapter/claudecode/claudecode.go`, `Current`). `Current` looks
only in the project directory named by encoding `dir` itself. The `handover`
and `takeover` skills (`plugins/turncost/skills/*/SKILL.md`) run
`turncost handover write` and `turncost handover take` with no `-dir`.

**Open:**

- **A handover written from a subdirectory fails or lands under the wrong
  key.** Claude Code names a session's transcript directory after the
  directory the session was started in, but its Bash tool keeps the working
  directory between commands (from Claude Code's Bash tool documentation, not
  checked in this review). After the session has `cd`'d into, say, `internal/`,
  the skill's `turncost handover write` runs there: `Current` looks for
  `<root>/<encoded …/internal>/*.jsonl`, finds nothing, and `write` fails with
  `no session transcript in …; pass -session`. Given `-session`, the note is
  stored under the subdirectory's key, while the statusline (keyed by its
  `cwd` field) and `/takeover` in the fresh session (started in the launch
  directory) look under the launch directory's key and do not see it.
  *Proposed:* make the skills pass the session's own directory (for example,
  `-dir` with the project root the harness exposes), or have the CLI walk up
  from the working directory to the nearest ancestor that has a project
  directory. *Why optional:* the plan's contract says `-dir` defaults to the
  working directory, and the code does exactly that. A failed write shows its
  error, and the skill stops, so nothing is lost silently.
