---
kind: change
area: model
state: open
---

# Prices are a hand-kept table

*Change, 2026-11-10.* `internal/pricing/pricing.go` holds list prices typed in
from the Claude API reference on 2026-11-10, Claude models only. On
2026-11-19 Sonnet 5.5's cache read was found at $0.20/M where the list says
$0.10/M (0.05× input, like Opus 5.5), and Fable 5.2 (440 requests) and Haiku
5.5 (75) were missing; all three fixed, Haiku 5.5 at its tier up to 100k.

**Open:**

- tiered prices (Haiku 5.5 is priced by prompt length), fast mode, batch
- prices for the models other harnesses use (DeepSeek, GPT)
- a price file the user can override instead of a Go map
- what to show on a subscription, where list price measures weight, not a bill
