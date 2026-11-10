---
name: benchpress
description: Write and keep the repository's workbench — one Markdown file per topic under the directory `.benchpress.toml` names, and its NEXT.md. Use when turning notes or findings into a topic, updating what a topic says, settling or planning one, deleting a finished one, or asked what to work on next.
---

# Keeping the workbench

The workbench holds what the author and agents are working through: bugs seen,
changes asked for, design questions, ideas. It is a working surface, not a
roadmap and not a source of truth: the repository's design documents hold
direction, and the source holds what actually runs.

Each topic is one file in the workbench directory (`dir` in `.benchpress.toml`,
`docs/workbench` by default). `NEXT.md` beside them is the order. Read only the
topics a task needs; `benchpress ls` lists them all in one screen, and
`benchpress ls --json` does so for a script.

## A topic

```markdown
---
kind: design
area: army
state: open
---

# Drafts strand, and the player cannot help them

*Design, from the 2026-10-13 playtest.* What the notes or the author said,
dated, in the author's words where they matter.

**Now:** what the code does today, with file and procedure landmarks.

**Open:**

- each question still to settle
```

- **The file name is the slug** and the topic's identity: lower case, digits
  and hyphens. Create one with `benchpress new "<title>" --kind K --area A`, or
  write the file by hand.
- **Front matter** is exactly `key: value` lines: `kind` and `area` from the
  configured lists, `state`, `plan` only for a planned topic, and `epic` only
  for a sub-topic (see *Epics*). Nothing else.
- **The title** is the one `# ` heading, first after the front matter.
- **Date what was said and what was measured.** A topic outlives the build it
  describes; a date tells the next reader how stale a line may be.
- **Now is checked, not remembered.** Recheck a topic's *Now* against the source
  before acting on it, and correct it when it has drifted.
- **One question, one place.** A question lives in one topic, not also in a
  design document or a second topic. Link instead:
  `[Drafts strand](drafts-strand.md)`.
- **Bugs need fixing, not discussing.** A bug topic says what was seen, the
  evidence, and what is known of the cause; label guesses as guesses.

## Epics

A subject too big for one topic — a whole feature with design, art, tooling
and tuning questions of its own — becomes an **epic**: an ordinary topic that
holds the overview and the questions that span it, with one sub-topic per
part. A sub-topic names it in its front matter, `epic: <slug>`, and keeps its
own kind, area and state. Create one with `benchpress new "<title>" --kind K
--area A --epic <slug>`.

- Epics are one level deep. A sub-topic cannot have sub-topics of its own.
- The epic links each sub-topic and says how they relate; each question still
  lives in one place, the epic or one sub-topic.
- A sub-topic is planned, settled and deleted like any topic. Delete the epic
  when its last sub-topic is gone, or sooner if what is left of it no longer
  hangs together.

## States

- `open` — still being discussed.
- `settled` — decided, waiting for a pass. Record what was settled and when,
  in the body, as *Settled (date):*.
- `planned` — a pass's plan has taken it: `benchpress state <slug> planned
  --plan docs/plans/<PLAN>.md`. Its details may now live in the plan; trim the
  topic to what the plan does not hold.

There is no done. **A finished or dropped topic is deleted**, not marked:
`benchpress rm <slug>`, which also takes it out of Next and lists the topics that
still link to it — fix or remove those links. Git keeps the history. When a pass
closes, its outcome moves into the repository's long-term documents and the
topics it took are deleted; `benchpress lint` reports a planned topic whose plan
is gone everywhere as `plan-gone`.

## Next

`NEXT.md` is the author's order and the only place order lives. Each item
begins with a link to a topic and may carry a short note after it. Change the
order only when the author asks; add or remove an item when the author asks or
when you delete a topic it names. `benchpress next add|rm|mv` does this without
disturbing the rest of the file.

## Before you finish

Run `benchpress lint` and fix every error it reports. Do not edit the
author's own notes or inbox files, wherever the repository keeps them; read
them and turn what is worth keeping into topics.
