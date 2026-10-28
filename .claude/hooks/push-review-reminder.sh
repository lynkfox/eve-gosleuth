#!/usr/bin/env bash
# Before a git push: remind the user to run /code-review when the last review
# (review-mark.sh's .git/claude-code-reviewed, holding the commit it saw) is
# more than MAX_COMMITS commits or MAX_LINES changed lines behind HEAD, or when
# no review is on record. It only reminds; the push goes ahead either way.
INPUT=$(cat)

CMD=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // empty')
grep -Eq '(^|[;&|(]|&&|\|\|)[[:space:]]*git([[:space:]]+-C[[:space:]]+[^[:space:]]+)?[[:space:]]+push\b' <<<"$CMD" || exit 0

MAX_COMMITS=5
MAX_LINES=500

repo="$CLAUDE_PROJECT_DIR"
gitdir=$(git -C "$repo" rev-parse --absolute-git-dir 2>/dev/null) || exit 0
marker="$gitdir/claude-code-reviewed"

reason=""
if [[ ! -f "$marker" ]]; then
  reason="no code review is on record"
else
  reviewed=$(sed -n 2p "$marker")
  if commits=$(git -C "$repo" rev-list --count "$reviewed..HEAD" 2>/dev/null); then
    lines=$(git -C "$repo" diff --shortstat "$reviewed" HEAD 2>/dev/null | grep -Eo '[0-9]+ (insertion|deletion)' | awk '{sum += $1} END {print sum + 0}')
    if (( commits > MAX_COMMITS )); then
      reason="$commits commits since the last code review"
    elif (( lines > MAX_LINES )); then
      reason="$lines lines changed since the last code review"
    fi
  else
    reason="the commit the last code review saw is gone (ie amended)"
  fi
fi
[[ -n "$reason" ]] || exit 0

msg="Reminder: $reason. Consider running /code-review before this push."
jq -n --arg m "$msg" '{
  systemMessage: $m,
  hookSpecificOutput: {hookEventName: "PreToolUse", additionalContext: ("Tell the user: " + $m)}
}'
