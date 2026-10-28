#!/usr/bin/env bash
# Before Edit, Write, NotebookEdit, or Bash: deny any change to a file listed in
# .claude/protected-files (the user's own words, ie Readme.md). Reading them, or
# mentioning them, is fine. File tools are checked by path.
# A Bash command is split into simple commands at ; & | and newlines, heredoc
# bodies set aside as text (unless fed to a shell), and one is denied only when
# it names a protected file as a path and writes to it: a redirect into it,
# sed/perl -i, tee, mv, rm, touch, truncate, cp/install/ln/dd onto it, or git
# checkout/restore/rm/mv/apply. A python/perl/ruby/node/PowerShell script, inline
# or in a heredoc, is denied when it quotes the file's path and writes something.
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
# the command without its heredoc bodies, which are text (a commit message, a
# file's new contents) rather than commands, unless the heredoc is fed to a shell
shell=$(awk '
  body {
    t = $0
    if (strip) sub(/^\t+/, "", t)
    if (t == delim) { body = 0; next }
    if (keep) print
    next
  }
  { print }
  match($0, /(^|[^<])<<-?[ \t]*["'"'"']?[a-z_][a-z0-9_]*/) {
    d = substr($0, RSTART, RLENGTH)
    strip = (d ~ /<<-/)
    sub(/^[^<]?<<-?[ \t]*["'"'"']?/, "", d)
    delim = d
    keep = ($0 ~ /(^|[ \t;|&(])(ba|da|z)?sh([ \t]|$)/)
    body = 1
  }' <<<"$cmd")

for p in "${protected[@]}"; do
  name=$(printf '%s' "$(basename "$p")" | lower | sed 's/[.[\*^$]/\\&/g')
  grep -q -- "$name" <<<"$cmd" || continue
  # the name as a path argument, not a word inside one
  end="(\$|[[:space:]\"')])"
  arg="(^|[[:space:]\"'/=>])$name$end"
  # bash only takes these as [[ =~ ]] patterns through a variable
  wrapper='^([({!]|sudo|env|command|exec|then|do|else|[a-z_][a-z0-9_]*=.*)$'
  onto="^[\"']?([^[:space:]]*/)?$name[\"')]*\$"

  # a script that quotes its path and writes
  grep -Eq "(^|[^a-z0-9_-])(python3?|perl|ruby|node|powershell(\.exe)?|pwsh)([[:space:]]|$)" <<<"$cmd" &&
    grep -Eq "[\"']([^\"'[:space:]]*/)?$name[\"']" <<<"$cmd" &&
    grep -Eq "(write|open\(|['\"][wa]\+?['\"]|set-content|out-file|add-content)" <<<"$cmd" && deny "$p"

  while IFS= read -r seg; do
    grep -Eq "$arg" <<<"$seg" || continue
    # a redirect into it
    grep -Eq ">[[:space:]]*[\"']?([^[:space:]\"']*/)?$name$end" <<<"$seg" && deny "$p"
    # the command word, past variable assignments and wrappers like sudo
    read -ra words <<<"$seg"
    i=0
    while (( i < ${#words[@]} )) && [[ "${words[i]}" =~ $wrapper ]]; do
      (( i++ ))
    done
    (( i < ${#words[@]} )) || continue
    verb=${words[i]#[({]}
    verb=${verb##*/}
    last=${words[-1]}
    case "$verb" in
      sed | perl) grep -Eq "[[:space:]](-[a-z]*i|--in-place)" <<<"$seg" && deny "$p" ;;
      tee | mv | rm | touch | truncate) deny "$p" ;;
      cp | install | ln) [[ "$last" =~ $onto ]] && deny "$p" ;;
      dd) grep -Eq "(^|[[:space:]])of=[\"']?([^[:space:]\"']*/)?$name$end" <<<"$seg" && deny "$p" ;;
      git) grep -Eq "[[:space:]](checkout|restore|rm|mv|apply)([[:space:]]|$)" <<<"$seg" && deny "$p" ;;
    esac
  done < <(tr ';&|' '\n' <<<"$shell")
done
exit 0
