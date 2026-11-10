---
kind: design
area: adapters
state: open
---

# Adapters for other harnesses

*Design, 2026-11-10.* The author: make the tool harness-agnostic, each harness
an adapter, "as all models/harnesses function on the same principles".

**Now:** only `internal/adapter/claudecode`. The CLI calls it directly; there
is no adapter interface yet.

**Open:**

- where Codex and OpenCode keep their sessions and what usage they record
  (unverified: OpenCode keeps a database, `OPENCODE_DB` in the dev shell)
- whether every harness reports cache reads and writes separately, and what
  to do when one does not
- the adapter interface, once a second adapter shows what it must hide
