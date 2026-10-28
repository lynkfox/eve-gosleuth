#!/usr/bin/env bash
# When the user runs /code-review, leave a marker in .git so push-guard.sh lets
# the next push to main through. push-reminder.sh removes it after a push, so
# every push needs its own review.
# Runs on UserPromptSubmit (the prompt starts with /code-review), on
# UserPromptExpansion (the expanded command is code-review), and after Claude
# runs the code-review skill at the user's request (PostToolUse on Skill).
# The Skill hook has no "if" filter, which did not match a forked skill run, so
# this checks the skill's name itself (a plugin skill is named plugin:code-review).
INPUT=$(cat)
if [[ $(printf '%s' "$INPUT" | jq -r '.tool_name // empty') == "Skill" ]]; then
  printf '%s' "$INPUT" | jq -e '.tool_input.skill // "" | test("(^|:)code-review$")' >/dev/null 2>&1 || exit 0
else
  printf '%s' "$INPUT" | jq -e '[.. | strings] | any(test("^\\s*/?code-review(\\s|$)"))' >/dev/null 2>&1 || exit 0
fi

gitdir=$(git -C "$CLAUDE_PROJECT_DIR" rev-parse --absolute-git-dir 2>/dev/null) || exit 0
{ date -u +%Y-%m-%dT%H:%M:%SZ; git -C "$CLAUDE_PROJECT_DIR" rev-parse HEAD 2>/dev/null; } > "$gitdir/claude-code-reviewed"
exit 0
