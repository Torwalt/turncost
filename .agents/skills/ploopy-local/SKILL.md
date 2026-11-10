---
name: ploopy-local
description: This repository's own rules for ploopy's skills — writing plans, carrying out units, reviewing, triage, landing, and working with ploopy here. Read the section named after the ploopy skill in use, alongside that skill.
---

# ploopy in this repository

ploopy's skills, which `ploopy skills` writes beside this one, are the same
in every repository. This file adds what is particular to this one, under a
heading per skill. ploopy hands each section to the sessions it runs with that
skill, after the skill; an agent working by hand reads it alongside the skill.
A rule here wins over the skill. A section left empty adds nothing.

## ploopy

Runs launch into a tmux window of their own (`launch = "tmux"`). The window's
shell loads the dev shell through direnv; a run started any other way has no
`go`, `benchpress` or `ploopy` on its path and fails its preflight's
`just verify`.

A worktree's `setup` links `.env` from the main checkout, approves its
`.envrc` and enters the dev shell once, which writes `.pre-commit-config.yaml`
and installs the commit-msg hook. Never copy `.pre-commit-config.yaml` by hand
or commit it.

## plan-writing

## plan-unit

## plan-review

## review-triage

## review-direct

## unit-rescue

## land-repair
