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

**Now:** a first backtest ran on 2026-11-19 as throwaway code outside the
repository: at every prompt of 102 interactive sessions it priced a reset in
hindsight (later carry against the reset estimate) and compared nudge rules.
Its results, and the constants fitted from them and from seven real
handovers, are in `docs/DESIGN.md` under *The readout*. There is no
counterfactual pricing in the repository.

Seen 2026-11-10 in session `6fd8e41f` (the costliest, $47): turn 15 opens a
new benchpress topic at 343k context right after commits, and the session
spends another ~$29 after it.

**Open:**

- whether the backtest becomes a command (`turncost backtest`), so the
  constants can be refitted as transcripts accumulate
- how to price the rebuild beyond the takeover: the files a new session
  re-reads count as new work today
- whether the result says task boundaries are worth automating or a cost
  readout is enough
