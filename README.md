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
450k · last turn $2.35 · step $0.090 (89% history) · reset pays in ~15 steps (<1 turn)
```

That is the context, what the last user turn cost, what the next step costs at
cache-read price and how much of it is history above the session's starting
context, and how many steps of that carry a reset (handover note plus a fresh
start) takes to pay back. When the cache has gone cold it shows
`450k · cold: next step $3.60 · last turn $2.35`, the price of re-writing the
context. Before the first request, or when the transcript is not there yet, it
prints nothing. The figures are numbers, not advice, and the reset estimate
rests on placeholder constants (`internal/signal`).

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

`-dir` defaults to the working directory; `-session` defaults to the most
recently modified transcript of that directory's project. Two sessions open in
the same directory at once can pick the wrong one: pass `-session`.

While a note waits for the project and the session has made no request yet
(right after `/clear`), the statusline shows
`handover waiting since 14:02 · /takeover` instead of nothing.

## Development

The dev shell is a Nix flake loaded by direnv. `just verify` runs the static
checks, `just test` the tests. See `AGENTS.md` and `docs/DESIGN.md`.
