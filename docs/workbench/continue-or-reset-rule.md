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
statusline shows: the last turn's cost (everything up to the next prompt), the
cold next turn, and the nudge, `↻ handover pays`, once twenty more requests of
carry (C − B at cache-read price) cost at least the reset estimate. Settled
2026-11-11: the author asked for a nudge "rather than just showing these
numbers". Settled 2026-11-19: the author wants the bar easy to read, in
English, with the nudge as its main signal; history and background figures
left it, and the rule and its constants were fitted to the backtest (see
`docs/DESIGN.md`, *The readout*).

**Open:**

- whether the nudge should also wait for a task boundary (a commit or push in
  the last turn), once the backtest says it helps
- which signals besides price per turn: cost growth rate, history share of a
  turn, turns since the last boundary event, cold-cache gaps
- one score or several separate readouts
- whether the rule needs the harness's model mix (Opus re-reads at 0.05× input,
  Sonnet at 0.1×)
