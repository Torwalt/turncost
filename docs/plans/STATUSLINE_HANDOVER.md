---
test: yes
context: docs/DESIGN.md README.md
---

# Statusline readout and handover

Status: active task document, written 2026-11-11 against `f3c15fc`.

This pass makes turncost usable live in Claude Code: it installs from the
flake or with `go install`, a statusline segment shows what the session's next
step costs and when a reset would pay, and a plugin ships two skills that hand
a session over through `/clear`, with a hook that warns before a handover
re-writes a cold cache.

## Decisions settled with the author (2026-11-11)

- **Statusline first.** Waybar, desktop notifications and a `Stop`-hook nudge
  are out of this pass.
- **Install** through the flake (`packages.default`, so
  `nix profile install github:Torwalt/turncost` works) and through
  `go install github.com/Torwalt/turncost/cmd/turncost@latest`; the repository
  is public. Wiring it into Claude Code must be obvious from the README.
- **No state for the readout.** The transcript is the state: the statusline
  parses the whole session on every call (the largest real transcript, 58 MB,
  parses in 0.14 s).
- **Numbers, not a verdict.** The segment shows figures; it never says
  "reset now".
- **Interactive sessions only.** The statusline only runs in them; nothing in
  this pass filters SDK sessions.
- **Harness-neutral core.** Only `internal/adapter/<harness>` knows a
  harness's formats, and that includes the JSON Claude Code pipes to a
  statusline command or a hook. An adapter interface is extracted in this pass.
- **Handover through `/clear`, never automatic.** `/handover` writes a note,
  the author types `/clear`, then `/takeover` loads it. Nothing is injected on
  `/clear` itself: a `/clear` often starts something unrelated.
- **Skill names** are `handover` and `takeover`, in a plugin named `turncost`
  (`/turncost:handover`, or `/handover` when no other command has the name).
- **Cold guard.** Typing `/handover` into a cold, large session is blocked
  once by a `UserPromptSubmit` hook that names the price; sending it again goes
  through.

## Facts about Claude Code this pass relies on

Read from the Claude Code docs on 2026-11-11 (code.claude.com/docs/en,
pages `statusline`, `hooks`, `plugins/components`, `plugins/create-marketplace`,
`skills`) and from the author's transcripts.

- **Statusline:** one `statusLine.command` per user; Claude Code pipes a JSON
  object to its stdin and shows its stdout. Fields used here:
  `transcript_path`, `session_id`, `cwd`. It re-runs after each assistant
  message, at session start (including after `/clear`), and when a warm prompt
  cache expires; updates are debounced at 300 ms. The transcript file may not
  exist yet at session start.
- **UserPromptSubmit hook:** stdin JSON carries `session_id`,
  `transcript_path`, `cwd` and `prompt`. A skill typed as a slash command
  arrives as typed (`/handover`, `/turncost:handover`, plus any arguments), not
  expanded. To block, print `{"decision":"block","reason":"…"}` to stdout and
  exit 0; the reason is shown to the user and nothing reaches the model.
- **Plugins:** a marketplace is `.claude-plugin/marketplace.json` at the
  repository root (`name`, `owner`, `plugins: [{name, source}]`); the plugin
  is a directory with `.claude-plugin/plugin.json` (`name` required),
  `skills/<name>/SKILL.md` and `hooks/hooks.json` (top-level `hooks` key, same
  shape as settings hooks). Users run `/plugin marketplace add Torwalt/turncost`
  and `/plugin install turncost@turncost`. A plugin cannot set `statusLine`.
- **`/clear`** starts a new session with a new transcript whose first entry is
  the `/clear` command (seen in the author's transcripts).
- **Project directories:** a transcript lives in
  `<root>/<encoded cwd>/<session-id>.jsonl`, where the encoded cwd is the
  absolute working directory with every character other than an ASCII letter
  or digit replaced by `-` (`/home/ada/repos/art-deal-flow.worktrees/capture-flow`
  → `-home-ada-repos-art-deal-flow-worktrees-capture-flow`).

## Contracts

### The readout (pure, `internal/signal`)

Computed from one `session.Session`, the current time and the price table.
*Main requests* are requests with `Sidechain` false; costs include sidechain
requests, everything else uses main requests only.

- **B**, baseline: prompt tokens (`Usage.Prompt()`) of the first main request
  (`signal.baseline` today).
- **C**, context: prompt tokens of the last main request.
- **p**: the price of the last main request's model. Unknown model: the
  readout carries C only and every price figure is absent.
- **TTL**: one hour if any main request wrote 1-hour cache, otherwise five
  minutes. **Write price** `w`: `CacheWrite1h` for a one-hour TTL,
  `CacheWrite5m` otherwise.
- **Cold**: now − time of the last main request > TTL.
- **User turns**: turns are grouped. A group starts at a `prompt` turn, or at
  a `command` turn that has requests; the `notification` and `continuation`
  turns after it join it until the next group starts. A `command` turn without
  requests (`/clear`, `/effort`) joins nothing and starts nothing. **Last
  turn** = cost of the last group with at least one request.
  **Steps per turn** = median count of main requests over groups with at least
  one.
- **Step** = C × p.CacheRead / 1e6. **Carry** = max(C − B, 0) × p.CacheRead /
  1e6. **History share** = carry / step.
- **Cold next step** = C × w / 1e6.
- **Reset estimate** R = handover + fresh, with
  handover = 3 × C × p.CacheRead / 1e6 + 5000 × p.Output / 1e6 (warm) or
  C × w / 1e6 + 2 × C × p.CacheRead / 1e6 + 5000 × p.Output / 1e6 (cold), and
  fresh = 2 × B × w / 1e6. The constants (three requests, 5,000 output tokens,
  twice the baseline to rebuild) are placeholders for the backtest; keep them
  as named constants in one place.
- **Payback steps** = ceil(R / carry) when carry > 0, otherwise absent.
  **Payback turns** = payback steps / steps per turn.

Worked example (Opus 5.5: read $0.20/M, 1-hour write $8/M, output $20/M;
B = 48,000; C = 450,000; one-hour TTL; last main request 10 minutes ago; last
group $2.35; steps per turn 20): step $0.090, carry $0.0804, history 89%;
handover $0.37, fresh $0.768, R $1.138; payback 15 steps, 0.75 turns. With the
last request 72 minutes ago instead: cold, cold next step $3.60, handover
$3.88.

### The statusline segment

Exact text, figures from the readout (`k` = thousands of tokens, integer):

- warm, carry > 0:
  `450k · last turn $2.35 · step $0.090 (89% history) · reset pays in ~15 steps (<1 turn)`
  — turns print as `<1 turn` below one, else `~N turns` rounded to the nearest
  integer (`~1 turn` for one).
- warm, carry = 0: `48k · last turn $1.50 · step $0.010`
- cold: `450k · cold: next step $3.60 · last turn $2.35`
- unknown model: `450k`
- no main request yet and no waiting handover: empty.
- a handover note waiting for this project while the session has no main
  request: `handover waiting since 14:02 · /takeover` (local time of the note).

`turncost statusline [-- command args…]` reads the statusline JSON from
stdin. With a command after `--`, it runs that command with the same stdin
bytes; when it prints anything, turncost prints its output with trailing
newlines trimmed, then ` · ` and the segment on its last line (the segment
alone when the command printed nothing, the command's output alone when the
segment is empty). The command's exit status is ignored. turncost never fails
the statusline: on any error of its own it prints whatever it has and exits 0.

### Handover notes (`internal/handover`)

- Directory: `$XDG_STATE_HOME/turncost/handovers`, default
  `~/.local/state/turncost/handovers`, then one directory per project named
  with the same encoding as Claude Code's project directories (above).
- One file per note: `<UTC time as 20061102T150405Z>-<first 8 of session id>.md`,
  holding `key: value` header lines, a line `---`, then the note body. Keys:
  `from` (session id), `harness`, `written` (RFC 3339), and once settled either
  `taken` (`<session id> <RFC 3339>`) or `dropped` (RFC 3339).
- **Waiting** note: the newest note of the project that is neither taken nor
  dropped. Writing a note marks older waiting notes of the project dropped, so
  at most one waits.
- `turncost handover write [-session ID] [-dir DIR]` reads the body from stdin,
  stores it, prints the file path and `next: /clear, then /takeover`.
- `turncost handover take [-session ID] [-dir DIR]` prints the waiting note's
  body and marks it taken by the session; with no waiting note it prints
  `no handover waiting for <dir>` to stderr and exits 1.
- `turncost handover drop [-dir DIR]` marks the waiting note dropped.
  `turncost handover ls [-dir DIR]` lists the project's notes with their state.
- `-dir` defaults to the working directory; `-session` defaults to the
  adapter's current session for that directory: the most recently modified
  transcript in its project directory. Two sessions open in the same directory
  at once can pick the wrong one; say so in the README.

### Cold guard (`turncost hook prompt`)

Reads the UserPromptSubmit JSON. It blocks only when all hold: the prompt,
trimmed, is `/handover` or `/turncost:handover`, alone or followed by
whitespace; the readout is cold; C − B ≥ 100,000 tokens; and no marker exists
for this session and this last-request time. Blocking writes the marker
(`$XDG_STATE_HOME/turncost/warned/<session id>`, holding the last main
request's time in RFC 3339) and prints the block decision with the reason:

`turncost: the cache is cold (idle 1h12m). This handover first re-writes 450k tokens ≈ $3.60; a fresh session starts at ≈ $0.38. Send /handover again to do it anyway, or /clear and start from git.`

(fresh start = B × w / 1e6; idle time rounded to the minute and printed as
`1h12m` or `45m`.) In every other case, including any error, it
prints nothing and exits 0. The 100,000-token threshold was chosen while
writing this plan, not by the author; keep it a named constant.

## Traps

- Never commit a real transcript. Fixtures under `testdata/` are written by
  hand from real shapes; statusline and hook payloads in tests too.
- The readout takes `now` as a parameter; nothing in `internal/signal` reads
  the clock.
- Tests that touch the handover store set `XDG_STATE_HOME` to a temporary
  directory (`t.Setenv`), never the real one.

## Stage 1 — Foundations

### 1.1 Adapter interface

**Goal:** the CLI reaches Claude Code only through a harness-neutral adapter
interface, which also answers "which transcript is the current session of
this directory".

**Start from:** `internal/adapter/claudecode/claudecode.go` (`DefaultRoot`,
`Discover`, `Find`, `Load`) and its callers in `cmd/turncost/main.go`
(`runSessions`, `runTurns`, `loadAll`). Define the interface in
`internal/adapter` with a way for the CLI to get the Claude Code adapter; add
`Current(root, dir string) (string, error)` to it, implemented for Claude Code
with the project-directory encoding from the preamble. `sessions` and `turns`
behave as before.

**Done when:** `rg 'claudecode\.' cmd/` finds at most the one place that
selects the adapter; a test covers `Current` against a temporary directory
with two transcripts of different modification times and the encoding of a
path holding `.` and `/`; `just verify` and `just test` pass.

### 1.2 Flake package and install instructions

**Goal:** turncost installs from the flake and with `go install`, and the
README says how.

**Start from:** `flake.nix` `packages` (it holds only `opencode`). Add
`default`, built with `buildGoModule` from `./.` with `vendorHash = null`
(standard library only) and `subPackages = [ "cmd/turncost" ]`. Rewrite the
README's *Use* section into *Install* (`nix profile install
github:Torwalt/turncost`, using it as a flake input, `go install …@latest`)
followed by the existing usage.

**Done when:** `nix build .#default && ./result/bin/turncost` prints the usage
text; `just verify` passes (`nixfmt --check` included).

## Stage 2 — The readout

### 2.1 Readout in `internal/signal`

**Goal:** a pure function computes the readout contract in the preamble from a
session, the current time and the price table.

**Start from:** `internal/signal/signal.go` (`Analyze`, `baseline`, `ttl`);
`internal/pricing/pricing.go` (`Lookup`, `Price`). Replace `ttl`'s
per-request guess with the session-wide TTL rule if `Analyze` can share it
without changing `turns` output for existing fixtures; otherwise leave
`Analyze` alone.

**Done when:** tests in `internal/signal` reproduce the worked example (warm
and cold) to the cent, cover turn grouping (a `/clear` command without
requests, notifications joining the prompt before them), an unknown model, a
session with no main request and carry = 0; `just test` passes.

### 2.2 `turncost statusline`

**Goal:** `turncost statusline [-- command…]` prints the segment from the
preamble for the session Claude Code names, optionally joined to the user's
existing statusline.

**Start from:** the statusline facts and the segment contract in the
preamble. Parsing the statusline JSON belongs in
`internal/adapter/claudecode`; formatting and the `--` wrapping in
`cmd/turncost`. Ignore handover notes here: the waiting form of the segment
comes with the handover store. Add a README section *Claude Code statusline*
with the `settings.json` snippet, alone and wrapping an existing command.

**Done when:** tests feed a hand-written statusline payload pointing at a
fixture transcript and check the exact segment for the warm, cold and empty
cases, the wrapped join (command output with and without a trailing newline,
a command that prints nothing, a command that fails) and a missing transcript
(exit 0); `echo '{}' | bin/turncost statusline` exits 0; `just test` passes.

## Stage 3 — Handover

### 3.1 Handover store and CLI

**Goal:** `turncost handover write|take|drop|ls` work as the handover-notes
contract says, and the statusline shows a waiting note in a session that has
made no request yet.

**Start from:** the handover-notes contract; `Current` from the adapter
interface for the `-session` default; the segment's waiting form. The store
is a new package `internal/handover` and knows no harness format.

**Done when:** tests (with `XDG_STATE_HOME` in a temporary directory) cover
write then take, take with nothing waiting (exit 1), a second write dropping
the first, drop, and the waiting form of the statusline segment for a
transcript holding only a `/clear`; `just test` passes.

### 3.2 Plugin with the handover and takeover skills

**Goal:** the repository is a Claude Code marketplace whose `turncost` plugin
ships the `handover` and `takeover` skills.

**Start from:** the plugin facts in the preamble. Write
`.claude-plugin/marketplace.json` (marketplace `turncost`, owner `Torwalt`),
`plugins/turncost/.claude-plugin/plugin.json` and the two skills. The skills
are generic: no reference to this repository, benchpress, ploopy or a
particular language, and they need only the `turncost` binary on PATH.

- `handover` writes, from what the session already knows, a note with these
  sections: goal; state (what is done, with commit hashes, and what is
  uncommitted); decisions and why; open questions; the next step; files to read
  first; dead ends not to retry. It may run `git status` and
  `git log --oneline -10`, but reads no other files: every extra request
  re-reads the whole context. It keeps the note under about 1,000 words,
  pipes it to `turncost handover write` through a quoted heredoc, and ends by
  telling the user to type `/clear`, then `/takeover`. If the repository keeps
  its own notes for durable findings, those go there, not only into the note.
- `takeover` runs `turncost handover take`, reads only the files the note
  says to read first, states in a few lines where the work stands and the next
  step, and waits for the user unless the invocation carried an instruction
  (`$ARGUMENTS`). With no note waiting it says so and stops.

Add a README section *Handover* (install the plugin, the three-step flow, the
same-directory limit). Add a `jq empty` check of the plugin's JSON files to
`just verify`.

**Done when:** `jq empty` passes on the three JSON files inside `just verify`;
both `SKILL.md` files have `name` and `description` front matter;
`rg -i 'benchpress|ploopy|hero' plugins/` comes back empty; `just verify`
passes.

### 3.3 Cold guard hook

**Goal:** `turncost hook prompt` implements the cold-guard contract, and the
plugin registers it as a `UserPromptSubmit` hook.

**Start from:** the cold-guard contract and the hook facts in the preamble.
Parsing the hook JSON belongs in `internal/adapter/claudecode`; the decision
uses the readout. Register `turncost hook prompt` in
`plugins/turncost/hooks/hooks.json`. Mention the guard in the README's
*Handover* section.

**Done when:** tests cover: blocking a cold large session with the exact
reason; passing the same prompt again (marker present); passing when warm,
when C − B is below the threshold, for any other prompt (including
`/handoverx` and `please /handover`), and for an unreadable transcript; the
hooks file passes `jq empty`; `just verify` and `just test` pass.

## Stage 4 — Close

### 4.1 Design document and workbench

Test: no

**Goal:** `docs/DESIGN.md` and the workbench say what this pass built.

**Start from:** `docs/DESIGN.md` (*Model*, *Adapters*, *Known limits*); the
workbench topics `live-watcher`, `handover-and-takeover-through-clear`,
`continue-or-reset-rule` and `harness-adapters`. Add to DESIGN.md: the
adapter interface, the readout and its placeholder constants, the handover
store as turncost's only state, and the Claude Code statusline, hook and
plugin facts the pass relies on. Delete the `live-watcher` topic with
`benchpress rm`, trim `handover-and-takeover-through-clear` to its open
questions and set it back to `open`, and update the *Now* of the other two.

**Done when:** `benchpress lint` and `just verify` pass; DESIGN.md mentions
`turncost statusline`, `turncost handover` and `turncost hook prompt`.

## For the author

- `turncost skill install` for other harnesses was discussed but is left out:
  without a second adapter, a skill elsewhere would have no session to read.
  It stays an open question in the handover topic.
- After landing: install with `nix profile install github:Torwalt/turncost`,
  run `/plugin marketplace add Torwalt/turncost` and
  `/plugin install turncost@turncost`, and set `statusLine.command` to
  `turncost statusline -- <your current statusline command>`.
