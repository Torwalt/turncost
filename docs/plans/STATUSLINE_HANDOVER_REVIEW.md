---
reviews: docs/plans/STATUSLINE_HANDOVER.md
test: no
review-base: d2dbc807d1e9ebf5c1e6f7c68ac61291ab593257
review-head: fcea995649393d152168b985543b766fa0569b5b
---

# Review of STATUSLINE_HANDOVER

Status: written by `ploopy review` on 2026-11-11 against
`fcea995649393d152168b985543b766fa0569b5b`. It reviews 8 landed units of
`docs/plans/STATUSLINE_HANDOVER.md`.

Only required fixes go into `docs/plans/STATUSLINE_HANDOVER_FIXES.md`: evidenced
defects, regressions, violated repository rules or unmet plan requirements.
Useful optional improvements go into one follow-up issue for this review pass,
using the session's persistence instructions. Unsupported concerns are dropped.
Zero fixes and zero optional findings is a successful review. Explain required
design choices and their alternatives under *For the author*.

One frontier session reviews the complete change `git diff
d2dbc807d1e9ebf5c1e6f7c68ac61291ab593257..fcea995649393d152168b985543b766fa0569b5b`
against the plan at the recorded tip, including fixes already applied. It checks
the units and their integration itself, without collecting sessions, and writes
the fix plan only when necessary.

## Stage 1 — Review

### 1.1 Review the branch against the plan

Review: direct
Agent: claude
Model: opus
Effort: high

**Goal:** evidence checked, duplicates merged, required fixes separated from
optional improvements and unsupported concerns dropped; a fix plan only for
required fixes, with any necessary design decisions resolved; optional findings
persisted together in one issue.

**Done when:** the review summary is committed under *For the author*, the fix
plan when necessary lints clean, and any optional findings are committed in one
follow-up document linked from the summary. No source or tests changed.


## For the author

Direct review, 2026-11-11, of `git diff
d2dbc807d1e9ebf5c1e6f7c68ac61291ab593257..fcea995649393d152168b985543b766fa0569b5b`
against `docs/plans/STATUSLINE_HANDOVER.md` at
`fcea995649393d152168b985543b766fa0569b5b`: all eight units (adapter interface,
flake package, readout, statusline, handover store, plugin and skills, cold
guard, design document and workbench) and how they fit together.

**Outcome: no required fixes**, so there is no fix plan. One optional finding is
recorded in
[statusline-handover-review-fcea99564939](../workbench/statusline-handover-review-fcea99564939.md).

### What was checked

- **Contracts against code.** `signal.Read` (`internal/signal/readout.go`)
  matches the readout contract: B and C from main requests, TTL, cold,
  grouping (a request-less command starts and joins nothing), step, carry,
  cold next step, the warm and cold handover, fresh, payback. The placeholder
  constants sit together in one `const` block. The statusline segment
  (`cmd/turncost/statusline.go`) produces each exact form, the waiting form
  and the `--` join. The handover store (`internal/handover`) has the
  file-name format, header keys, at most one waiting note and the
  `write|take|drop|ls` behaviour. The cold guard (`cmd/turncost/hook.go`)
  applies its four conditions, the per-session marker holding the last request
  time, the exact reason text and the 100,000-token named constant.
- **Boundaries.** `rg 'claudecode\.' cmd/` is empty, and `adapter.Default()`
  is the one selector. The statusline and hook JSON are parsed only in
  `internal/adapter/claudecode`. `internal/handover` knows no harness format,
  and `internal/signal` takes `now` as a parameter.
- **Plugin.** The marketplace, plugin and hooks JSON pass `jq empty` inside
  `just verify`. Both skills have `name`/`description` front matter and stay
  generic. `rg -i 'benchpress|ploopy|hero' plugins/` is empty.
- **Checks run.** `just verify` and `just test` pass.
  `nix build .#default` builds.
- **Docs.** DESIGN.md names all three commands. `live-watcher` is deleted, the
  handover topic is trimmed to its open questions and set to `open`, and the
  *Now* sections of the other two topics are updated. The README has *Install*,
  *Claude Code statusline* and *Handover*, including the same-directory limit.

### Optional findings

One finding, kept in the linked topic: a handover written after the session has
`cd`'d into a subdirectory fails to find its transcript, or is stored under the
subdirectory's key where the statusline and `/takeover` do not look. It is
optional because the code follows the plan's own contract (`-dir` defaults to
the working directory), and the failure is reported to the user, not silent.

### Dropped concerns

- `turncost hook` with a subcommand other than `prompt` exits 2, which a
  `UserPromptSubmit` hook treats as blocking. The plugin registers only
  `turncost hook prompt`, and the contract's "every case exits 0" covers that
  command, not misuse.
- `Analyze` keeps its per-request TTL guess. The plan allowed leaving it
  alone.
- The handover skill sets `disable-model-invocation` while its description
  says when to use it; this is wording, not a defect. The README describes the
  note store and the plugin in two sections, which is taste.
- Marker and settled-note cleanup, the guard threshold and the placeholder
  constants are already open questions in
  `handover-and-takeover-through-clear`. Recording them again would duplicate
  them.

No design decisions were needed.
