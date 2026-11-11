---
kind: design
area: adapters
state: open
---

# Adapters for other harnesses

*Design, 2026-11-10.* The author: make the tool harness-agnostic, each harness
an adapter, "as all models/harnesses function on the same principles".

**Now:** only `internal/adapter/claudecode`. The CLI reaches it through the
`adapter.Adapter` interface (`internal/adapter/adapter.go`): transcript
discovery and loading, the current session of a directory, the project key, and
parsing the JSON the harness pipes to a statusline command or a hook.
`adapter.Default()` is the one place that picks the harness. The interface was
cut from one adapter, so it is a guess until a second shows what it must hide.

**Open:**

- where Codex and OpenCode keep their sessions and what usage they record
  (unverified: OpenCode keeps a database, `OPENCODE_DB` in the dev shell)
- whether every harness reports cache reads and writes separately, and what
  to do when one does not
- the adapter interface, once a second adapter shows what it must hide:
  whether the statusline and hook payloads, the project key and "current
  session" generalise
