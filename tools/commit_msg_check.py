from __future__ import annotations

import re
import sys
from dataclasses import dataclass
from pathlib import Path


ATTRIBUTION_PATTERNS: tuple[tuple[str, str], ...] = (
    (r"^\s*Claude-Session\s*:", "a Claude-Session trailer"),
    (r"^\s*(Session|Agent|Assistant)-(Id|Link|Url)\s*:", "a session identifier trailer"),
    (
        r"^\s*Co-authored-by\s*:.*\b("
        r"claude|anthropic|openai|gpt|copilot|codex|opencode|deepseek|cursor|"
        r"aider|devin|gemini|\bbot\b|\[bot\]"
        r")",
        "an AI co-author trailer",
    ),
    (r"Generated with \[?Claude", "a Claude Code generation line"),
    (r"^\s*\U0001F916", "a robot-emoji attribution line"),
)

MAX_BULLETS = 3


@dataclass(frozen=True)
class Violation:
    line: int
    message: str

    def format(self) -> str:
        return f"commit message line {self.line}: {self.message}"


def strip_comments(message: str) -> list[str]:
    return [line for line in message.split("\n") if not line.startswith("#")]


def check_attribution(lines: list[str]) -> list[Violation]:
    violations = []

    for number, line in enumerate(lines, start=1):
        for pattern, description in ATTRIBUTION_PATTERNS:
            if re.search(pattern, line, re.IGNORECASE):
                violations.append(Violation(number, f"remove {description}"))
                break

    return violations


def check_bullets(lines: list[str]) -> list[Violation]:
    bullets = [
        number
        for number, line in enumerate(lines, start=1)
        if re.match(r"^\s*[-*+]\s+\S", line)
    ]

    if len(bullets) <= MAX_BULLETS:
        return []

    return [
        Violation(
            bullets[MAX_BULLETS],
            f"{len(bullets)} bullet points; the body takes at most {MAX_BULLETS}",
        )
    ]


def check(message: str) -> list[Violation]:
    lines = strip_comments(message)
    return check_attribution(lines) + check_bullets(lines)


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print("usage: commit_msg_check.py <path-to-commit-message>")
        return 2

    violations = check(Path(argv[1]).read_text(encoding="utf-8"))
    for violation in violations:
        print(violation.format())

    if violations:
        print(f"{len(violations)} commit message violation(s)")
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
