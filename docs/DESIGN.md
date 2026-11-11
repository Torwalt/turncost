# Design

## The question

A session's next turn costs roughly *context × cache-read price + new tokens*.
Context only grows, so each turn costs more than the last even when the task
is the same size. A reset trades that growing re-read for a one-off cost: a
fresh session's starting context, a handover note, and whatever the agent has
to re-read to rebuild what mattered. turncost exists to put numbers on both
sides of that trade, first after the fact, then live.

The motivating measurement (2026-11-10, the author's Claude Code transcripts
2026-10-12 to 2026-11-10, ~28k requests, ~$1,840 at list prices):

- Cache hit rate is 97–99% on every model; caching is not the problem.
- Cache reads are 51% of the cost, 1-hour cache writes 28%, output 21%.
- Interactive sessions are 71% of the cost. Sessions over 200 requests
  average 243k context and peak at 441k; under 50 requests they average 65k.
- The top 10% of sessions hold half the cost.

## Model

`internal/session` is harness-neutral:

- **Session**: one transcript; id, harness, project, branch, harness version.
- **Turn**: everything between one user-side message and the next: a typed
  prompt, a slash command, a background-task notification, or a compaction's
  continuation. A turn owns its requests and the events they caused.
- **Request**: one API call: time, model, usage split into uncached input,
  cache read, 5-minute and 1-hour cache writes, and output. Subagent requests
  are marked `Sidechain` and join the turn they ran in.
- **Event**: a task-boundary marker seen in the turn's tool calls: commit,
  push, publish, edit, slash command.

`internal/pricing` prices a request from a hand-kept table per model.
`internal/signal` derives per-turn figures from a session: cost, running
total, context at turn start, history cost, gap and cold cache.
`signal.Read` derives the live readout of a session at a given time: context,
step cost, carry, cold next step, and the reset estimate with its payback in
steps. Its reset constants are placeholders for the backtest.
`turncost statusline` formats that readout for Claude Code's statusline: the
adapter parses the JSON Claude Code pipes in (`Adapter.ParseStatusline`), the
transcript is parsed whole on every call, and nothing is kept between calls.

`internal/handover` stores the notes a session leaves for the next one: one
file per note under a project directory in `$XDG_STATE_HOME/turncost`, a header
of `key: value` lines, `---`, the body. At most one note per project waits;
writing drops the older ones, taking records the session that took it. The
store treats the project as an opaque key; the adapter derives it from a
working directory (`Adapter.ProjectKey`). The statusline shows a waiting note
while its session has no main request yet.

## Adapters

One package per harness under `internal/adapter/`, producing `session.Session`.
Only an adapter knows its harness's files. The CLI reaches them through the
`adapter.Adapter` interface in `internal/adapter` (default root, discover,
find, load, `Current`: the transcript of the current session of a
directory, and `ProjectKey`); `adapter.Default()` is the one place that
selects a harness.

### Claude Code

- Transcripts: `$CLAUDE_CONFIG_DIR/projects/<project-dir>/<session-id>.jsonl`,
  default `~/.claude/projects`; subagents in
  `<session-id>/subagents/*.jsonl` beside it.
- A project directory is named after the session's working directory with every
  character other than an ASCII letter or digit replaced by `-`. The current
  session of a directory is the most recently modified transcript there; two
  sessions open in one directory at once can be confused.
- One JSON event per line. Assistant events carry `message.usage` with
  `cache_creation.ephemeral_{5m,1h}_input_tokens`; `entrypoint` is `cli` for
  interactive use and `sdk-cli` for SDK runs such as ploopy.
- **One API response is logged as several lines, one per content block, each
  repeating the usage.** Requests are deduplicated by `requestId`; counting
  lines inflates every figure.
- User events that start a turn: non-meta, with text. Tool results,
  `<local-command-…>` output and `isMeta` lines do not.
- The format is internal and undocumented; it changes between versions. The
  adapter keeps `version` so a break can be dated.
- Claude Code deletes transcripts after `cleanupPeriodDays` (30 by default).
  Raise it to keep history for analysis.

## What is deterministic

Everything above: token counts, costs, context per turn, gaps, events and any
counterfactual priced from them ("had the session reset at turn N"). Judging
whether a new prompt opens a new topic is not; if that is ever wanted, it is an
optional layer beside the deterministic signals, never under them.

## Known limits

- Commit and push detection is a substring match on Bash commands, so a
  heredoc that contains `git commit` counts. See the workbench.
- Prices are hand-entered (2026-11-10) and ignore tiered pricing and batch
  discounts.
- History cost is a lower bound: it prices extra context at the cache-read
  rate and ignores the 1-hour writes that put it there.
