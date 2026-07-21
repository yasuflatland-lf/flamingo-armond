#!/usr/bin/env bash
# Fail when a committed doc cites a source location by line number.
# Rule: docs/doc-organization.md § "Describe code locations by structure, not line number".
# Fenced regions are skipped so quoted compiler / linter output keeps passing.
set -euo pipefail

if [[ $# -eq 0 ]]; then
  roots=(docs .claude/rules)
else
  roots=("$@")
fi

# Point-in-time snapshots are exempt: plan documents record what the tree looked
# like on a given day, so rewriting their citations is churn, not maintenance.
exclude_re='(^|/)docs/superpowers/plans/'

files=()
while IFS= read -r f; do
  [[ "$f" =~ $exclude_re ]] && continue
  files+=("$f")
done < <(find "${roots[@]}" -type f -name '*.md' | sort)

if [[ ${#files[@]} -eq 0 ]]; then
  echo "OK: no Markdown files to scan under ${roots[*]}"
  exit 0
fi

hits=$(awk '
  FNR == 1 { fenced = 0; fence = "" }
  {
    line = $0
    # Fence delimiters: a line whose first non-space run is ``` or ~~~ (3+).
    if (match(line, /^[ \t]*(`{3,}|~{3,})/)) {
      delim = substr(line, RSTART, RLENGTH)
      sub(/^[ \t]*/, "", delim)
      marker = substr(delim, 1, 1)
      if (!fenced) { fenced = 1; fence = marker; next }
      if (marker == fence) { fenced = 0; fence = ""; next }
      next
    }
    if (fenced) next
    if (line ~ /\.(go|ts|tsx|sql|graphql):[0-9]+/) {
      printf "%s:%d: %s\n", FILENAME, FNR, line
    }
  }
' "${files[@]}")

if [[ -n "$hits" ]]; then
  echo "ERROR: line-number citation in committed docs (see docs/doc-organization.md § \"Describe code locations by structure, not line number\")." >&2
  echo "Reference code by symbol name instead. Quoted tool output belongs inside a fenced code block." >&2
  echo "$hits" >&2
  exit 1
fi

echo "OK: no line-number citations outside fenced blocks (${#files[@]} files scanned)"
