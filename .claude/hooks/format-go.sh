
#!/usr/bin/env bash
set -euo pipefail

INPUT=$(cat)

FILE=$(printf '%s' "$INPUT" |
  jq -r '.tool_input.file_path // empty')

if [[ "$FILE" == *.go && -f "$FILE" ]]; then
  gofmt -w "$FILE"
fi

