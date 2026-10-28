#!/usr/bin/env bash
# Before Edit, Write, NotebookEdit, or Bash: deny any change to a file listed in
# .claude/protected-files (the user's own words, ie Readme.md). Reading them is
# fine. File tools are checked by path; a Bash command is denied when it names a
# protected file and would write to it: a redirect into it, sed/perl -i, tee,
# mv, cp, rm, touch, truncate, git checkout/restore, or a script that writes.
# A command that writes without naming the file (ie git reset --hard) is not caught.
INPUT=$(cat)

repo="$CLAUDE_PROJECT_DIR"
list="$repo/.claude/protected-files"
[[ -f "$list" ]] || exit 0
mapfile -t protected < <(grep -Ev '^[[:space:]]*(#|$)' "$list" | sed 's/[[:space:]]*$//')
(( ${#protected[@]} )) || exit 0

deny() {
  jq -n --arg r "$1 is protected (.claude/protected-files): Claude may read it but never change it. Ask the user to make the change themselves." '{
    hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny", permissionDecisionReason: $r}
  }'
  exit 0
}

tool=$(printf '%s' "$INPUT" | jq -r '.tool_name // empty')
lower() { tr '[:upper:]' '[:lower:]'; }

if [[ "$tool" != "Bash" ]]; then
  path=$(printf '%s' "$INPUT" | jq -r '.tool_input.file_path // .tool_input.notebook_path // empty')
  [[ -n "$path" ]] || exit 0
  [[ "$path" = /* ]] || path="$repo/$path"
  path=$(realpath -m "$path" | lower)
  for p in "${protected[@]}"; do
    [[ "$path" == "$(realpath -m "$repo/$p" | lower)" ]] && deny "$p"
  done
  exit 0
fi

cmd=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // empty' | lower)
for p in "${protected[@]}"; do
  name=$(printf '%s' "$(basename "$p")" | lower | sed 's/[.[\*^$]/\\&/g')
  grep -q -- "$name" <<<"$cmd" || continue
  # a redirect into it
  grep -Eq ">[[:space:]]*[\"']?[^[:space:]\"';|&]*$name" <<<"$cmd" && deny "$p"
  # in-place edits, file moves and removals, or git putting back an older copy
  grep -Eq "(^|[^a-z0-9_-])(sed|perl)([[:space:]][^|;&]*)?[[:space:]]-[a-z]*i" <<<"$cmd" && deny "$p"
  grep -Eq "(^|[^a-z0-9_-])(tee|mv|cp|rm|touch|truncate|dd|install|ln|git[[:space:]]+(checkout|restore|rm|mv|apply))([[:space:]]|$)" <<<"$cmd" && deny "$p"
  # a script that opens it for writing
  grep -Eq "(^|[^a-z0-9_-])(python3?|perl|ruby|node|powershell(\.exe)?|pwsh)([[:space:]]|$)" <<<"$cmd" &&
    grep -Eq "(write|open\(|['\"][wa]\+?['\"]|set-content|out-file|add-content)" <<<"$cmd" && deny "$p"
done
exit 0
