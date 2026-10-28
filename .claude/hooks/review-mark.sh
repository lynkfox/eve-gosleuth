#!/usr/bin/env bash
# Whenever a review runs, record when and the commit it saw in
# .git/claude-code-reviewed, for push-review-reminder.sh.
# Runs on UserPromptSubmit (the prompt starts with /code-review or
# /security-review), on UserPromptExpansion (the expanded command is one), and
# after Claude runs one of those skills or the reviewer agent (PostToolUse on
# Skill and Agent).
# The Skill and Agent hooks have no "if" filter, which did not match a forked
# skill run, so this checks the name itself (a plugin skill is plugin:code-review).
INPUT=$(cat)
case $(printf '%s' "$INPUT" | jq -r '.tool_name // empty') in
  Skill) printf '%s' "$INPUT" | jq -e '.tool_input.skill // "" | test("(^|:)(code|security)-review$")' >/dev/null 2>&1 || exit 0 ;;
  Agent) printf '%s' "$INPUT" | jq -e '.tool_input.subagent_type == "reviewer"' >/dev/null 2>&1 || exit 0 ;;
  *) printf '%s' "$INPUT" | jq -e '[.. | strings] | any(test("^\\s*/?(code|security)-review(\\s|$)"))' >/dev/null 2>&1 || exit 0 ;;
esac

gitdir=$(git -C "$CLAUDE_PROJECT_DIR" rev-parse --absolute-git-dir 2>/dev/null) || exit 0
{ date -u +%Y-%m-%dT%H:%M:%SZ; git -C "$CLAUDE_PROJECT_DIR" rev-parse HEAD 2>/dev/null; } > "$gitdir/claude-code-reviewed"
exit 0
