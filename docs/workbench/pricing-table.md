---
kind: change
area: model
state: open
---

# Prices are a hand-kept table

*Change, 2026-11-10.* `internal/pricing/pricing.go` holds list prices typed in
from the Claude API reference on 2026-11-10, Claude models only.

**Open:**

- tiered prices (Haiku 5.5 is priced by prompt length), fast mode, batch
- prices for the models other harnesses use (DeepSeek, GPT)
- a price file the user can override instead of a Go map
- what to show on a subscription, where list price measures weight, not a bill
