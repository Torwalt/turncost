---
kind: idea
area: signals
state: open
---

# Usage report across sessions

*Idea, 2026-11-11.* The author: expand turncost "into a general analysis tool
later on", for example "your cache rate for Opus is good, but for Sonnet it's
not so good", "some form of usage or token analysis of your Claude".

**Now:** `turncost sessions` lists the costliest sessions with their hit rate
(`cmd/turncost/main.go`, `runSessions`). The founding measurement in
`docs/DESIGN.md` (cost split by reads, writes and output; interactive against
SDK; cost by session length) was computed by hand.

**Open:**

- which breakdowns: per model, per project, interactive (`cli`) against SDK
  (`sdk-cli`), per week
- whether a report flags anomalies (a model's hit rate well below the others)
  or only shows figures
