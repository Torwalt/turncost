---
kind: design
area: watcher
state: open
---

# Handover and takeover through /clear

*Design, 2026-11-11.* Built: the `handover` and `takeover` skills, the note
store, the statusline's waiting-note segment and the cold guard. `docs/DESIGN.md`
holds what they do; this topic keeps what is still open.

**Open:**

- the same skills for other harnesses (Codex `/new` plays `/clear`'s part)
- whether the cold guard should cover every prompt into a large cold session,
  not just `/handover`
- the cold guard's 100,000-token threshold was chosen while writing the plan,
  not by the author; the backtest should set it
- the handover's cost is an estimate (three requests, 5,000 output tokens); a
  taken note records the old and the new session, so real resets can be priced
  and the placeholders fitted
- notes in `warned/` and settled notes in `handovers/` are never cleaned up
- two sessions open in one directory at once can confuse `-session`'s default
