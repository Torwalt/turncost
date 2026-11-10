---
kind: idea
area: signals
state: open
---

# Backtest resets at task boundaries

*Idea, 2026-11-10.* Before building a watcher, price what resetting would have
saved: replay each recorded session with a reset after every commit, publish
or push (and, separately, after every cold-cache gap), re-price the following
turns at a fresh context plus a handover, and compare with what was paid.

**Now:** the events and per-turn costs exist (`session.Event`,
`signal.TurnCost`). There is no counterfactual pricing.

Seen 2026-11-10 in session `6fd8e41f` (the costliest, $47): turn 15 opens a
new benchpress topic at 343k context right after commits, and the session
spends another ~$29 after it.

**Open:**

- how to price the reset's rebuild: a flat handover size, or the re-reads the
  agent actually did at the session's start
- whether the result says task boundaries are worth automating or a cost
  readout is enough
