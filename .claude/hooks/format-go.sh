#!/usr/bin/env bash
# After Edit/Write: gofmt the edited file. After a Bash command that names a
# .go file (sed -i, heredocs, scripts): gofmt every changed or new .go file.
set -euo pipefail

INPUT=$(cat)
cd "$CLAUDE_PROJECT_DIR"

FILE=$(printf '%s' "$INPUT" | jq -r '.tool_input.file_path // empty')
if [[ -n "$FILE" ]]; then
  if [[ "$FILE" == *.go && -f "$FILE" ]]; then
    gofmt -w "$FILE"
  fi
  exit 0
fi

CMD=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // empty')
[[ "$CMD" == *.go* ]] || exit 0

{ git diff --name-only; git ls-files --others --exclude-standard; } |
  grep '\.go$' | sort -u | while read -r f; do
    [[ -f "$f" ]] && gofmt -w "$f"
  done
