# AGENTS.md

turncost reads the transcripts coding-agent harnesses write, prices every turn
of a session at API list prices, and derives signals for one question: is the
next turn of this session still worth its context, or would a fresh session be
cheaper? It is a spike: Claude Code is the first harness adapter, the
signals are being found, and a live watcher comes after them.
`docs/DESIGN.md` holds the model, the boundaries and what is known about the
data.

## Start here

* **`just verify`** after editing; **`just test`** when behaviour changed.
* **`just fmt`** owns formatting; never format by hand.
* **Do the whole task**: say what you left undone and why.
* **Report failures as failures**, with the output.

## Code

* Go, standard library only unless a dependency earns its place; say why in
  the commit that adds it.
* `internal/session` is the harness-neutral model. Adapters under
  `internal/adapter/<harness>` turn a harness's files into it; nothing outside
  an adapter knows a harness's file format.
* `internal/signal` and `internal/pricing` are pure functions over the model:
  same transcripts, same numbers. An LLM-backed signal, if one comes, is an
  optional layer that never feeds the deterministic ones.
* Every adapter is tested against a small fixture in its `testdata/`, written
  by hand from real transcript shapes. Never commit a real transcript: they
  hold private prompts and code.

## Environment

* The dev shell is the flake; direnv loads it with `.env`. Every tool comes
  from the shell: add a tool to `flake.nix`, never install it globally.
* Secrets live only in the ignored `.env`. Never commit or print them.

## Workbench and plans

* `docs/workbench/` holds the topics and `NEXT.md`, the author's order; keep it
  through the `benchpress` skill and run `benchpress lint` before finishing.
* A pass gets one task document in `docs/plans/`, written with the
  `plan-writing` skill and run by `ploopy`, one unit per fresh session. Only
  ploopy writes `<PLAN>.state.json`. Repository rules for ploopy's skills go in
  `ploopy-local`.

## Commits

* Concise subject, an optional body of at most three bullets.
* No AI attribution, `Co-authored-by` or session trailers; preserve the
  configured Git identity. The commit-msg hook enforces this.
* Keep history linear: "merge master" means rebase onto master.
