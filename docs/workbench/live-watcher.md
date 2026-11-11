---
kind: design
area: watcher
state: planned
plan: docs/plans/STATUSLINE_HANDOVER.md
---

# Live readout in the Claude Code statusline

*Design, 2026-11-10; settled with the author 2026-11-11.* Tell the author
during a session, not after it.

**Settled (2026-11-11):**

- The statusline comes first; the author: "let's do status line first". Waybar
  and desktop notifications wait until a second harness makes a neutral
  surface worth it.
- Installable through the flake and through `go install`; the repository is
  public. Wiring it into Claude Code must be obvious from the README.
- `turncost statusline` prints its own segment and, given `-- <command>`, runs
  the user's existing statusline on the same input and joins the two, since
  Claude Code runs only one statusline command and a plugin cannot set it.
- No state of its own: the whole transcript is parsed on every call. Measured
  2026-11-11: the largest transcript (58 MB) parses in 0.14 s.
- Numbers, not a verdict: context, last turn's cost, the price of the next
  step and how much of it is history, when a reset would pay back, and the
  price of a cold cache. Turn-over-turn cost was rejected as the signal: in
  session `127ca753` turn 1 cost $7.80 at 48k context and turn 2 $1.25 at 285k,
  because cost follows the number of requests in a turn.
- Interactive sessions only. That comes for free: the statusline runs only in
  interactive sessions.

Claude Code re-runs the statusline when a warm prompt cache expires, so a
cold marker appears without a refresh timer (statusline docs, read
2026-11-11).

**Open:**

- a nudge at the end of a turn (`Stop` hook or desktop notification), once the
  backtest says the numbers deserve one
