#!/usr/bin/env bash
# Before a git push to main: refuse it unless /code-review ran since the last
# push (review-mark.sh leaves .git/claude-code-reviewed, holding the commit it
# reviewed) and main has at most MAX_COMMITS_AFTER_REVIEW commits beyond that
# commit, so a review's fix commits go out with it without another review.
# Pushes that carry no .go, .js, .html, or .css changes need no review.
INPUT=$(cat)

CMD=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // empty')
grep -Eq '(^|[;&|(]|&&|\|\|)[[:space:]]*git([[:space:]]+-C[[:space:]]+[^[:space:]]+)?[[:space:]]+push\b' <<<"$CMD" || exit 0

repo="$CLAUDE_PROJECT_DIR"
branch=$(git -C "$repo" rev-parse --abbrev-ref HEAD 2>/dev/null) || exit 0
# a push to main: from the main branch, or naming main as the target
[[ "$branch" == "main" ]] || grep -Eq '(^|[[:space:]:])main([[:space:]]|$)' <<<"$CMD" || exit 0

MAX_COMMITS_AFTER_REVIEW=5

gitdir=$(git -C "$repo" rev-parse --absolute-git-dir 2>/dev/null) || exit 0
reason="This push to main carries code that has not had a code review. Ask the user to run /code-review first; push once it is done and its fixes are committed."
marker="$gitdir/claude-code-reviewed"
if [[ -f "$marker" ]]; then
  reviewed=$(sed -n 2p "$marker")
  # the commits on main the review did not see; an unknown commit (ie amended away) counts as none seen beyond it
  after=$(git -C "$repo" rev-list --count "$reviewed..main" 2>/dev/null) || after=0
  (( after <= MAX_COMMITS_AFTER_REVIEW )) && exit 0
  reason="main has $after commits since the last code review, more than $MAX_COMMITS_AFTER_REVIEW. Ask the user to run /code-review again before pushing."
fi

# what the push would send: the commits main has that its upstream lacks
upstream=$(git -C "$repo" rev-parse --verify -q 'main@{u}' 2>/dev/null) || upstream=""
if [[ -n "$upstream" ]]; then
  git -C "$repo" diff --name-only "$upstream" main 2>/dev/null | grep -Eq '\.(go|js|html|css)$' || exit 0
fi

jq -n --arg r "$reason" '{
  hookSpecificOutput: {
    hookEventName: "PreToolUse",
    permissionDecision: "deny",
    permissionDecisionReason: $r
  },
  systemMessage: "Push to main blocked: run /code-review first."
}'
