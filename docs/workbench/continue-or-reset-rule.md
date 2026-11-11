---
kind: design
area: signals
state: open
---

# When does continuing stop being worth it?

*Design, from the founding conversation, 2026-11-10.* The author: "one strong
signal is probably looking at one session and seeing the API price per next
iteration. E.g. you do the first prompt and it costs 0.1$. The second prompt
costs 0.5$ … You could do simple (maybe even complex) calcs here to determine
whether its worth it to continue. We may even find more such signals."

**Now:** `turncost turns` prints per-turn cost, running total, context at turn
start and history cost (`internal/signal/signal.go`, `Analyze`).
`signal.Read` (`internal/signal/readout.go`) derives the live readout the
statusline shows: the last turn's cost and its history share, background
follow-ups apart, the cold next turn, and the reset estimate with its payback
in turns like the last one. It shows numbers, never a verdict,
and its reset constants (`HandoverRequests`, `HandoverOutputTokens`,
`FreshRebuild`) are placeholders until the backtest fits them.

A first sketch of the trade, to test against the transcripts:

- *Carry cost* of turn n ≈ (context_n − baseline) × read price × requests in
  the turn. It grows with every turn.
- *Reset cost* ≈ a handover's output + a fresh context write + the re-reads
  the new session needs to rebuild what mattered.
- Reset pays when the carry cost over the turns still to come exceeds the
  reset cost. The turns still to come are unknown; the backtest can tell how
  long a session usually runs after a boundary.

**Open:**

- which signals besides price per turn: cost growth rate, history share of a
  turn, turns since the last boundary event, cold-cache gaps
- one score or several separate readouts
- whether the rule needs the harness's model mix (Opus re-reads at 0.05× input,
  Sonnet at 0.1×)
