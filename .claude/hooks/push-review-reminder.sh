#!/usr/bin/env bash
# Before a git push: remind the user to run /code-review when the last review
# (review-mark.sh's .git/claude-code-reviewed, holding the commit it saw) is
# more than MAX_COMMITS commits or MAX_LINES changed lines behind HEAD, or when
# no review is on record. It only reminds; the push goes ahead either way.
# A rebase or merged PR gives the reviewed commit a new hash, leaving the old one
# off the branch, where counting from it reaches back to where they forked. So
# when it is off the branch, this finds its copy and moves the marker there.
INPUT=$(cat)

CMD=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // empty')
grep -Eq '(^|[;&|(]|&&|\|\|)[[:space:]]*git([[:space:]]+-C[[:space:]]+[^[:space:]]+)?[[:space:]]+push\b' <<<"$CMD" || exit 0

MAX_COMMITS=5
MAX_LINES=500

repo="$CLAUDE_PROJECT_DIR"
gitdir=$(git -C "$repo" rev-parse --absolute-git-dir 2>/dev/null) || exit 0
marker="$gitdir/claude-code-reviewed"

# find_copy prints the newest commit on this branch that is the given commit under
# a new hash: the same files (a squash merge, a reworded commit), or the same author
# date and subject (a rebase, which keeps both even when the files differ).
find_copy() {
  local tree at subject
  IFS=$'\t' read -r tree at subject < <(git -C "$repo" log -1 --format=$'%T\t%at\t%s' "$1")
  git -C "$repo" log --format=$'%H\t%T\t%at\t%s' HEAD |
    T=$tree A=$at S=$subject awk -F'\t' '$2 == ENVIRON["T"] || ($3 == ENVIRON["A"] && $4 == ENVIRON["S"]) {print $1; exit}'
}

reason=""
if [[ ! -f "$marker" ]]; then
  reason="no code review is on record"
else
  reviewed=$(sed -n 2p "$marker")
  if ! git -C "$repo" cat-file -e "$reviewed^{commit}" 2>/dev/null; then
    reason="the commit the last code review saw is gone"
  else
    if ! git -C "$repo" merge-base --is-ancestor "$reviewed" HEAD 2>/dev/null; then
      copy=$(find_copy "$reviewed")
      if [[ -n "$copy" ]]; then
        reviewed=$copy
        { sed -n 1p "$marker"; printf '%s\n' "$reviewed"; } > "$marker.new" && mv "$marker.new" "$marker"
      else
        reason="the commit the last code review saw is not on this branch"
      fi
    fi
    if [[ -z "$reason" ]]; then
      commits=$(git -C "$repo" rev-list --count "$reviewed..HEAD" 2>/dev/null)
      lines=$(git -C "$repo" diff --shortstat "$reviewed" HEAD 2>/dev/null | grep -Eo '[0-9]+ (insertion|deletion)' | awk '{sum += $1} END {print sum + 0}')
      if (( commits > MAX_COMMITS )); then
        reason="$commits commits since the last code review"
      elif (( lines > MAX_LINES )); then
        reason="$lines lines changed since the last code review"
      fi
    fi
  fi
fi
[[ -n "$reason" ]] || exit 0

msg="Reminder: $reason. Consider running /code-review before this push."
jq -n --arg m "$msg" '{
  systemMessage: $m,
  hookSpecificOutput: {hookEventName: "PreToolUse", additionalContext: ("Tell the user: " + $m)}
}'
