---
name: handover
description: Write a handover note for the next session, so work can continue after /clear. Use when the user asks to hand the session over, to save where things stand before clearing, or types /handover.
disable-model-invocation: true
allowed-tools: Bash(git status:*), Bash(git log:*), Bash(turncost handover write:*)
---

Write a handover note from what this session already knows, store it with
`turncost handover write`, and tell the user how to continue. Extra context
requests are expensive: every one re-reads the whole conversation.

## Gather

You may run `git status` and `git log --oneline -10` to get commit hashes and
the uncommitted state right. Read no other files and run no other commands to
prepare the note; write it from memory of the session.

## The note

Markdown, under about 1,000 words, with these sections in this order. Write
for a reader who has none of this conversation: say what a name refers to
instead of assuming it is known. Leave out a section's content, not the
section, when there is nothing to say (write "none").

1. **Goal**: what the work is for, in a few lines.
2. **State**: what is done, with commit hashes; what is uncommitted, by file.
3. **Decisions**: what was decided and why, so the next session does not
   reopen it.
4. **Open questions**: what is unresolved and who has to answer.
5. **Next step**: the one thing to do first, concretely.
6. **Read first**: the files the next session should read, most important
   first, each with a few words on why. Only what it needs: it reads these and
   nothing else.
7. **Dead ends**: what was tried and failed, so it is not retried.

If the repository keeps its own notes for durable findings (a notes file,
docs, a workbench), put those findings there as well; the handover note is
for the next session only and is not a place to keep knowledge.

## Store it

Pipe the note to `turncost handover write` through a quoted heredoc, so
nothing in the note is expanded by the shell:

```sh
turncost handover write <<'HANDOVER_NOTE'
# Handover

…the note…
HANDOVER_NOTE
```

Pick a terminator that does not appear in the note. The command prints the
path of the file; if it fails, show the error and stop.

## Finish

End by telling the user, in one or two lines, that the note is saved and to
type `/clear`, then `/takeover`. Do nothing else: do not clear the session,
and do not start new work.
