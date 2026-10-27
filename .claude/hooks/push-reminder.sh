#!/usr/bin/env bash
# After a git push: a review-fix commit (subject "Review fixes: ...") ends the
# cycle with a /clear reminder; any other commit gets a /code-review reminder.
subject=$(git -C "$CLAUDE_PROJECT_DIR" log -1 --format=%s 2>/dev/null) || exit 0

if [[ "$subject" == "Review fixes:"* ]]; then
  msg="Review fixes pushed. Time to /clear: make sure anything worth keeping is in CLAUDE.md, docs/features.md, or memory first."
else
  msg="Pushed \"$subject\". Run /code-review on it next."
fi

jq -n --arg m "$msg" '{
  systemMessage: $m,
  hookSpecificOutput: {hookEventName: "PostToolUse", additionalContext: ("Reminder for the user: " + $m)}
}'
