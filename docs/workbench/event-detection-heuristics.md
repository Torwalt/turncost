---
kind: bug
area: adapters
state: open
---

# Commit detection counts heredocs that mention git

*Bug, seen 2026-11-10.* The founding session wrote a test fixture through a
heredoc containing `git commit -m x && git push`; `turncost turns` reports
that turn as `commit×2 push×2` though nothing was committed.

**Now:** `toolEvents` in `internal/adapter/claudecode/claudecode.go` matches
the substring `git commit` / `git push` anywhere in a Bash command.

**Open:**

- match the command word rather than a substring, or read the tool result for
  git's own output (`[branch abc1234]`)
