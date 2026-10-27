#!/usr/bin/env bash
# After a git push: a review-fix commit (subject "Review fixes: ...") ends the
# cycle with a /clear reminder; any other push that changed .go, .js, .html,
# or .css files gets a /code-review reminder.
# Says nothing unless a command really ran git push and it sent something.
INPUT=$(cat)

CMD=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // empty')
grep -Eq '(^|[;&|(]|&&|\|\|)[[:space:]]*git([[:space:]]+-C[[:space:]]+[^[:space:]]+)?[[:space:]]+push\b' <<<"$CMD" || exit 0

OUT=$(printf '%s' "$INPUT" | jq -r '[.tool_response.stdout?, .tool_response.stderr?, (.tool_response | strings)] | map(select(. != null)) | join("\n")' 2>/dev/null)
if grep -Eq 'Everything up-to-date|\[rejected\]|^(fatal|error):' <<<"$OUT"; then
  exit 0
fi
# The branch must now match its upstream, or the push didn't land.
head=$(git -C "$CLAUDE_PROJECT_DIR" rev-parse HEAD 2>/dev/null) || exit 0
upstream=$(git -C "$CLAUDE_PROJECT_DIR" rev-parse '@{u}' 2>/dev/null) || exit 0
[[ "$head" == "$upstream" ]] || exit 0

subject=$(git -C "$CLAUDE_PROJECT_DIR" log -1 --format=%s 2>/dev/null) || exit 0

if [[ "$subject" == "Review fixes:"* ]]; then
  msg="Review fixes pushed. Time to /clear: make sure anything worth keeping is in CLAUDE.md, docs/features.md, or memory first."
else
  # Only code needs a review: skip pushes that touch no .go, .js, .html, or
  # .css file. The pushed range comes from git's "old..new" line; without one
  # (a new branch) fall back to the last commit.
  range=$(grep -Eo '\b[0-9a-f]{7,40}\.\.\.?[0-9a-f]{7,40}\b' <<<"$OUT" | head -n1)
  [[ -n "$range" ]] || range="HEAD~1..HEAD"
  git -C "$CLAUDE_PROJECT_DIR" diff --name-only "$range" 2>/dev/null | grep -Eq '\.(go|js|html|css)$' || exit 0
  msg="Pushed \"$subject\". Run /code-review on it next."
fi

jq -n --arg m "$msg" '{
  systemMessage: $m,
  hookSpecificOutput: {hookEventName: "PostToolUse", additionalContext: ("Reminder for the user: " + $m)}
}'
