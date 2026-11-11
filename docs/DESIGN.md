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

## The readout

`signal.Read(session, now, lookup)` derives the live readout of one session; it
is pure, and `now` is a parameter. *Main requests* are those with `Sidechain`
false; costs include sidechain requests, everything else uses main requests.

- **B**, baseline: prompt tokens of the first main request. **C**, context:
  prompt tokens of the last. **p**: the price of the last request's model; for
  an unknown model the readout carries C only.
- **TTL**: one hour if any main request wrote 1-hour cache, else five minutes.
  **w**: the matching cache-write price. **Cold**: now − last main request > TTL.
- **Turn**: the user's message up to the agent's reply. A `prompt` turn, or a
  `command` turn that has requests, starts one; `continuation` turns after it
  join it. A request-less command (`/clear`, `/effort`) and a prompt stopped
  before any request change nothing. **Last turn** is the cost of the last
  turn with a request, subagents included. **Last turn history** is what its
  main requests paid to re-read context above the baseline:
  Σ max(Cᵢ − B, 0) × cache-read / 1e6.
- **Background**: the cost of requests that `notification` turns (a finished
  background task or monitor) set off after the last turn. The agent reacts to
  these after it has replied; they are not the user's turn, and turncost cannot
  tell a useful reaction from a redundant one. In the author's 40 costliest
  sessions of 2026-10-12 to 2026-11-11 they were 79 turns and 8.7% of the cost.
- **Cold next turn** (`ColdNext`) = C × w / 1e6: what the next turn pays first
  to re-write the context.
- **Reset estimate** R = handover + fresh. Handover is `HandoverRequests` (3)
  cache-read passes over C plus `HandoverOutputTokens` (5,000) of output; cold,
  the first pass is a cache write. Fresh is `FreshRebuild` (2) × B × w / 1e6,
  the new session's starting context twice over. These three constants are
  placeholders for the backtest and live in one block in
  `internal/signal/readout.go`. **Payback turns** = R / last turn history when
  that is positive: how many turns like the last one a reset pays back in.
  `FreshStart` = B × w / 1e6 is what a fresh session starts at.
- **Reset pays** (the nudge): last turn history ≥ R, and not cold. If the next
  turn is like the last and does not need this context, resetting is already
  cheaper. Whether it needs the context is the user's call; the nudge says only
  that the price favours a reset.

Requests (API calls, one per tool round trip) are how cost accrues, but the
readout no longer shows them: the user thinks in turns, and turn size varies
too much (1 to over 100 requests) for a per-request figure to say anything.

Worked example (Opus 5.5, read $0.20/M, 1-hour write $8/M, output $20/M;
B 48k, C 450k, last request 10 minutes ago, a last turn of ten requests at
450k): last turn $0.90, history $0.804, handover $0.37, fresh $0.768,
R $1.138, payback 1.4 turns, no nudge (fifteen such requests, history
$1.206, would nudge). Idle 72 minutes instead: cold next turn $3.60,
handover $3.88.

The readout started as numbers only; on 2026-11-11 the author asked for a
nudge "rather than just showing these numbers", hence *reset pays*.

## Commands that read it

- `turncost statusline [-- command args…]` formats the readout as one segment
  for Claude Code's statusline, short enough to share a line with the user's
  own: `ctx 76% · turn $1.75 (hist $0.71)`, where `ctx` is the share of the
  context window left (`context_window.remaining_percentage`, rounded down, the
  way the rate limits beside it read; the context in thousands of tokens,
  `241k`, when the harness does not report it), then ` · bg +$0.40` when background
  requests followed the turn, then ` · cold +$4.78` (cyan) when cold or
  ` · ↻ reset pays` (bold yellow) on the nudge; the context alone for an
  unknown model; and
  empty before the first main request. With a command after `--` it runs it on
  the same stdin and joins its output and the segment with ` · `. It never
  fails the statusline: on its own errors it prints what it has and exits 0.
  The transcript is parsed whole on every call (the largest real one, 58 MB,
  takes 0.14 s); nothing is kept between calls.
- `turncost handover write|take|drop|ls` manages the notes below.
- `turncost hook prompt` is the cold guard, a `UserPromptSubmit` hook. It blocks
  `/handover` or `/turncost:handover` (alone or followed by whitespace) when the
  readout is priced and cold and C − B ≥ `guardCarry` (100,000 tokens, chosen
  while planning, not by the author), once per session and last-request time:
  a marker `$XDG_STATE_HOME/turncost/warned/<session id>` holds that time, and
  the second send passes. It prints the block decision with the price of
  re-writing the context and the cost of a fresh start, and in every other case,
  including any error, prints nothing and exits 0. A session without a usable id
  or an unwritable marker is never blocked.

## State

The handover store, `internal/handover`, is turncost's only state, apart from
the guard's markers. Everything else is derived from transcripts on each call.

- Notes live in `$XDG_STATE_HOME/turncost/handovers` (default
  `~/.local/state/turncost/handovers`), one directory per project, one file per
  note: `<UTC 20061102T150405Z>-<first 8 of session id>.md`, `key: value` header
  lines (`from`, `harness`, `written`, and once settled `taken` as `<session>
  <time>` or `dropped`), a line `---`, the body.
- The waiting note is the newest one neither taken nor dropped; writing drops
  the older ones, so at most one waits. Taking records the new session, so the
  cost of a real reset can be measured later.
- The store treats the project as an opaque key; the adapter derives it from a
  working directory. `-session` defaults to the adapter's current session of the
  directory: the most recently modified transcript of its project, which two
  sessions open in one directory at once can confuse.
- The statusline shows `handover waiting since 14:02 · /takeover` (note's local
  time) while a note waits for the project and the session has no main request.
- Nothing is injected on `/clear`: a `/clear` often starts something unrelated.
  The author types `/handover`, `/clear`, `/takeover`.

`plugins/turncost` is the Claude Code plugin (marketplace at
`.claude-plugin/marketplace.json`): the `handover` and `takeover` skills, generic
prompts that drive `turncost handover write` and `take`, and the hook above.

## Adapters

One package per harness under `internal/adapter/`, producing `session.Session`.
Only an adapter knows its harness's files, and that includes the JSON the
harness pipes to a statusline command or a hook. The CLI reaches them through
the `adapter.Adapter` interface in `internal/adapter`:

- `Name`, `DefaultRoot`, `Discover(root)`, `Find(root, id)`, `Load(path)`:
  locate and parse transcripts.
- `Current(root, dir)`: the transcript of the current session of a directory.
- `ProjectDir(root, dir)`: the directory a working directory's sessions belong
  to, itself or its nearest ancestor with sessions. The handover commands use
  it, because the agent's shell may have moved into a subdirectory.
- `ProjectKey(dir)`: the name of the directory's project, the same for every
  session run there.
- `ParseStatusline(data)`: the session a statusline payload names, as a
  `session.Ref`.
- `ParsePrompt(data)`: the session and the prompt as typed, from a
  `UserPromptSubmit` payload. Whether a prompt is `/handover` is decided in the
  CLI, not the adapter.

`adapter.Default()` is the one place that selects a harness. The interface was
cut from a single adapter.

### Claude Code

- Transcripts: `$CLAUDE_CONFIG_DIR/projects/<project-dir>/<session-id>.jsonl`,
  default `~/.claude/projects`; subagents in
  `<session-id>/subagents/*.jsonl` beside it.
- A project directory is named after the session's working directory with every
  character other than an ASCII letter or digit replaced by `-`
  (`/home/ada/repos/a.b/c` → `-home-ada-repos-a-b-c`). The handover store uses
  the same name for its project directories.
- One JSON event per line. Assistant events carry `message.usage` with
  `cache_creation.ephemeral_{5m,1h}_input_tokens`; `entrypoint` is `cli` for
  interactive use and `sdk-cli` for SDK runs such as ploopy.
- **One API response is logged as several lines, one per content block, each
  repeating the usage.** Requests are deduplicated by `requestId`; counting
  lines inflates every figure.
- User events that start a turn: non-meta, with text. Tool results,
  `<local-command-…>` output and `isMeta` lines do not.
- `/clear` starts a new session with a new transcript whose first entry is the
  `/clear` command.
- The format is internal and undocumented; it changes between versions. The
  adapter keeps `version` so a break can be dated.
- Claude Code deletes transcripts after `cleanupPeriodDays` (30 by default).
  Raise it to keep history for analysis.

What the statusline, hook and plugin work relies on (Claude Code docs, read
2026-11-11; **not exercised in a live session**, only against hand-written
payloads):

- **Statusline**: one `statusLine.command` per user. Claude Code pipes a JSON
  object to its stdin (`transcript_path`, `session_id`,
  `context_window.remaining_percentage`, and `workspace.project_dir`, else
  `cwd`, are used) and shows
  its stdout. It re-runs after each assistant message, at session start
  (including after `/clear`) and when a warm prompt cache expires, debounced at
  300 ms; the transcript may not exist yet at session start. It only runs in
  interactive sessions, so SDK sessions need no filter.
- **UserPromptSubmit hook**: stdin JSON carries `session_id`, `transcript_path`,
  `cwd` and `prompt`. A skill typed as a slash command arrives as typed
  (`/handover`, `/turncost:handover`, plus arguments), not expanded. To block,
  print `{"decision":"block","reason":"…"}` and exit 0: the reason is shown to
  the user and nothing reaches the model. Matchers are ignored for this event.
- **Plugins**: a marketplace is `.claude-plugin/marketplace.json` at the
  repository root; a plugin is a directory with `.claude-plugin/plugin.json`,
  `skills/<name>/SKILL.md` and `hooks/hooks.json` (a top-level `hooks` key, the
  shape of settings hooks). Users run `/plugin marketplace add Torwalt/turncost`
  and `/plugin install turncost@turncost`. A plugin cannot set `statusLine`,
  which is why the statusline is wired in `settings.json` by hand.

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
- The reset estimate's constants and the guard's threshold are placeholders;
  the backtest should fit them.
- Settled notes and guard markers are never cleaned up.
