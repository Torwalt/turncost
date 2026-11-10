---
kind: design
area: watcher
state: open
---

# Live watcher: statusline and prompt-time nudge

*Design, 2026-11-10.* Tell the author during a session, not after it.

**Now:** nothing live. Claude Code gives a statusline script and hooks a JSON
payload on stdin with `session_id` and `transcript_path`, so a watcher can
read the same transcript the adapter reads.

Proposed shape:

- statusline segment: context, this turn's cost against a fresh session's,
  a cold-cache marker
- `UserPromptSubmit` hook: one line when context is large and the last turn
  made a boundary event, never a block
- a `handover` command or skill that writes a note so a reset loses little

**Open:**

- incremental parsing, so each statusline call reads only new lines
- whether the nudge should ever block
- how the same watcher fits other harnesses
