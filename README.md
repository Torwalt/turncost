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

## Development

The dev shell is a Nix flake loaded by direnv. `just verify` runs the static
checks, `just test` the tests. See `AGENTS.md` and `docs/DESIGN.md`.
