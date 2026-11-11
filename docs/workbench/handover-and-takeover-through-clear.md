---
kind: design
area: watcher
state: planned
plan: docs/plans/STATUSLINE_HANDOVER.md
---

# Handover and takeover through /clear

*Design, 2026-11-11.* When the readout says a reset pays, the author hands the
session over and continues in a fresh context. The author: the path is
`/clear`, "because clear is something people use more often, and usually I also
use clear", so no second session is spawned.

**Settled (2026-11-11):**

- Two skills, shipped in a Claude Code plugin: `/handover` in the old session
  writes a note through `turncost handover write`; `/takeover` in the new one
  loads it through `turncost handover take`. The author rejected `/continue` as
  too generic.
- No automatic injection on `/clear`. The author: "sometimes clear is also, you
  know, you just want to start a new thing"; an age-based heuristic "will
  probably break in a lot of cases". Keep it explicit, "power usery", but
  seamless: the statusline shows a waiting note while the new session has made
  no request yet.
- The note is the only state turncost owns; transcripts hold everything else.
  It records the old session and, once taken, the new one, so the cost of a
  real reset can be measured.
- Cold cache: a handover in a cold, large session first re-writes the whole
  context (450k on Opus 5.5 ≈ $3.60). A check inside the skill comes too late,
  because the request that loads the skill already pays it. A
  `UserPromptSubmit` hook blocks `/handover` once with the price; sending it
  again goes through.

Measured 2026-11-11: a warm handover at 450k on Opus 5.5 costs about
$0.30–0.40 (one re-read per request at $0.20/M plus a few thousand output
tokens).

**Open:**

- the same skills for other harnesses (Codex `/new` plays `/clear`'s part)
- whether the cold guard should cover every prompt into a large cold session,
  not just `/handover`
