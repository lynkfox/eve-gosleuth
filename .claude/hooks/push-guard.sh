#!/usr/bin/env bash
# Before a git push to main: refuse it unless /code-review ran since the last
# push (review-mark.sh leaves .git/claude-code-reviewed). Pushes that carry no
# .go, .js, .html, or .css changes need no review.
INPUT=$(cat)

CMD=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // empty')
grep -Eq '(^|[;&|(]|&&|\|\|)[[:space:]]*git([[:space:]]+-C[[:space:]]+[^[:space:]]+)?[[:space:]]+push\b' <<<"$CMD" || exit 0

repo="$CLAUDE_PROJECT_DIR"
branch=$(git -C "$repo" rev-parse --abbrev-ref HEAD 2>/dev/null) || exit 0
# a push to main: from the main branch, or naming main as the target
[[ "$branch" == "main" ]] || grep -Eq '(^|[[:space:]:])main([[:space:]]|$)' <<<"$CMD" || exit 0

gitdir=$(git -C "$repo" rev-parse --absolute-git-dir 2>/dev/null) || exit 0
[[ -f "$gitdir/claude-code-reviewed" ]] && exit 0

# what the push would send: the commits main has that its upstream lacks
upstream=$(git -C "$repo" rev-parse --verify -q 'main@{u}' 2>/dev/null) || upstream=""
if [[ -n "$upstream" ]]; then
  git -C "$repo" diff --name-only "$upstream" main 2>/dev/null | grep -Eq '\.(go|js|html|css)$' || exit 0
fi

jq -n '{
  hookSpecificOutput: {
    hookEventName: "PreToolUse",
    permissionDecision: "deny",
    permissionDecisionReason: "This push to main carries code that has not had a code review. Ask the user to run /code-review first; push once it is done and its fixes are committed."
  },
  systemMessage: "Push to main blocked: run /code-review first."
}'
