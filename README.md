# turncost

When is a coding-agent session no longer worth continuing?

Every turn of an agent session re-sends the whole conversation. Prompt caching
makes that cheap per token, but not free: at 300k tokens of context a turn
costs several times what it did at the start, and most of it pays for history
the next task may not need. turncost reads the session transcripts your
harness already writes, prices every turn, and shows where the cost went, so
you can see when a reset would have paid off and, later, be told so live.

It is an early spike. Claude Code is the first harness; others come as
adapters.

## Install

With Nix, into your profile:

```sh
nix profile install github:Torwalt/turncost
```

Or as an input of your own flake, using `turncost.packages.${system}.default`:

```nix
inputs.turncost.url = "github:Torwalt/turncost";
```

Or with Go:

```sh
go install github.com/Torwalt/turncost/cmd/turncost@latest
```

From a checkout, `just build` writes `bin/turncost`.

## Use

```sh
turncost sessions -n 20 -since 168h   # the most expensive sessions
turncost turns 6fd8e41f               # one session, turn by turn
```

`turns` prints, for each turn: its context size when it started, its cost, the
*history* share (the tokens above the session's starting context, at cache-read
price, a lower bound on what carrying the history cost), the running total,
the gap since the previous turn (`cold` when the gap outlived the cache), and
the commits, pushes, publishes and edits it made.

Costs are at API list prices from `internal/pricing`. On a subscription they
measure weight, not a bill.

Transcripts are read from `$CLAUDE_CONFIG_DIR/projects` or
`~/.claude/projects`; `-root` points elsewhere. Nothing leaves your machine.

## Claude Code statusline

`turncost statusline` reads the JSON Claude Code pipes to a statusline command
and prints one segment about the session it names, parsing the whole
transcript on each call (the largest real one takes about 0.14 s):

```
241k · turn $1.75 (hist $0.71)
571k · turn $3.26 (hist $2.47) · ↻ reset pays
597k · turn $3.26 (hist $2.47) · cold +$4.78
```

A *turn* is your message up to the agent's reply, subagents included. The
segment shows the context, what your last turn cost, and how much of that paid
for re-reading history above the session's starting context. `↻ reset pays`
(yellow) appears when that history cost at least as much as a reset would (a
handover note plus a fresh start): if your next turn is like the last one and
does not need this context, resetting is already cheaper. `cold +$4.78` (cyan)
is what the next turn pays first to re-write a context whose cache has
expired. `bg +$0.40` is what the agent spent reacting to finished background
tasks or monitors after its reply. Before the first request, or when the
transcript is not there yet, it prints nothing. `NO_COLOR` turns the colours
off. The reset estimate rests on placeholder constants (`internal/signal`).

Put turncost on your `PATH` (see Install), then in `~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "turncost statusline"
  }
}
```

To keep a statusline you already have, pass its command after `--`. It gets
the same JSON on stdin; its output (trailing newlines trimmed) comes first,
then ` · ` and the segment on the last line. Its exit status is ignored.

```json
{
  "statusLine": {
    "type": "command",
    "command": "turncost statusline -- ~/.claude/my-statusline.sh"
  }
}
```

Claude Code runs the command through a shell, so the wrapped command may carry
its own arguments and quoting. turncost never fails the statusline: on an
error of its own it prints what it has and exits 0.

## Handover notes

`turncost handover` keeps the note a session leaves for the next one, for a
handover through `/clear`:

```sh
turncost handover write [-session ID] [-dir DIR]   # note on stdin; prints its path
turncost handover take  [-session ID] [-dir DIR]   # prints the waiting note, marks it taken
turncost handover drop  [-dir DIR]                 # marks the waiting note dropped
turncost handover ls    [-dir DIR]                 # the project's notes and their state
```

Notes are plain files, `<UTC time>-<session id>.md` with `key: value` header
lines, `---` and the body, under
`$XDG_STATE_HOME/turncost/handovers/<project>/` (default
`~/.local/state/turncost/handovers`). Writing a note drops the project's older
waiting notes, so at most one waits. `take` with nothing waiting prints
`no handover waiting for <dir>` and exits 1.

`-dir` defaults to the working directory. Either way the note belongs to the
nearest directory, that one or above, that has sessions, so a handover written
after the agent's shell moved into a subdirectory still lands where the next
session looks. `-session` defaults to the most recently modified transcript of
that directory's project. Two sessions open in
the same directory at once can pick the wrong one: pass `-session`.

While a note waits for the project and the session has made no request yet
(right after `/clear`), the statusline shows
`handover waiting since 14:02 · /takeover` instead of nothing.

## Handover

A plugin for Claude Code hands a session over through `/clear`, on top of the
notes above. Put turncost on your `PATH` (see Install), then in Claude Code:

```
/plugin marketplace add Torwalt/turncost
/plugin install turncost@turncost
```

The flow has three steps, and nothing happens on its own:

1. `/handover` makes the session write a note from what it already knows: goal,
   state with commit hashes, decisions, open questions, the next step, files to
   read first and dead ends. It runs only `git status` and `git log` and
   stores the note with `turncost handover write`.
2. Type `/clear`. The statusline shows `handover waiting since 14:02 ·
   /takeover` in the new session.
3. `/takeover` loads the note (`turncost handover take`), reads only the files
   it names, says where the work stands and waits. Arguments, as in
   `/takeover carry on with the next step`, are followed as an instruction.

The skills are `/turncost:handover` and `/turncost:takeover` when another
command has the same name. They only need the `turncost` binary on `PATH`.
Nothing is injected on `/clear` itself, because a `/clear` often starts
something unrelated.

The plugin also registers a cold guard, `turncost hook prompt`, on
`UserPromptSubmit`. Typing `/handover` into a session whose cache has gone cold
and whose context is at least 100,000 tokens above its start is blocked once,
with the price: writing the note first re-writes the whole context at cache-write
price, while a fresh session starts far cheaper. Send `/handover` again to do it
anyway, or `/clear` and start from git. Every other prompt, and any error,
passes silently.

A note belongs to the directory the session was started in, and the session
that writes or takes it is the most recently modified transcript of that
directory. Two sessions open
in the same directory at once can confuse the two.

## Development

The dev shell is a Nix flake loaded by direnv. `just verify` runs the static
checks, `just test` the tests. See `AGENTS.md` and `docs/DESIGN.md`.
