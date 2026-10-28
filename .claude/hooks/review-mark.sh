#!/usr/bin/env bash
# When the user runs /code-review, leave a marker in .git so push-guard.sh lets
# the next push to main through. push-reminder.sh removes it after a push, so
# every push needs its own review.
# Runs on UserPromptSubmit (the prompt starts with /code-review) and on
# UserPromptExpansion (the expanded command is code-review).
INPUT=$(cat)
printf '%s' "$INPUT" | jq -e '[.. | strings] | any(test("^\\s*/?code-review(\\s|$)"))' >/dev/null 2>&1 || exit 0

gitdir=$(git -C "$CLAUDE_PROJECT_DIR" rev-parse --absolute-git-dir 2>/dev/null) || exit 0
{ date -u +%Y-%m-%dT%H:%M:%SZ; git -C "$CLAUDE_PROJECT_DIR" rev-parse HEAD 2>/dev/null; } > "$gitdir/claude-code-reviewed"
exit 0
