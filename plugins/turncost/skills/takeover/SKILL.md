---
name: takeover
description: Pick up the handover note a previous session left for this directory. Use at the start of a fresh session after /clear, when the user types /takeover or asks to continue from a handover.
disable-model-invocation: true
allowed-tools: Bash(turncost handover take:*)
---

Continue work from the note a previous session left, with as little extra
reading as possible.

## Load the note

Run:

```sh
turncost handover take
```

It prints the waiting note and marks it taken by this session. If it prints
`no handover waiting for …` and exits non-zero, say so in one line and stop:
do not look for notes elsewhere and do not start any work.

## Orient

Read only the files the note lists under its "read first" section, and
nothing else. Do not re-verify the note's claims by exploring the repository;
if something it says is plainly contradicted by a file you read, say so.

Then state in a few lines where the work stands and what the next step is.

## Continue

The invocation's arguments are: $ARGUMENTS

If they are empty, wait for the user: do not start the next step on your own.
If they carry an instruction, follow it, taking the note as its context.
