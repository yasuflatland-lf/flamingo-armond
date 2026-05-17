#!/usr/bin/env bash
set -euo pipefail

# Check 1: presence — every area CLAUDE.md exists
for d in backend frontend schema docs; do
  if [[ ! -f "$d/CLAUDE.md" ]]; then
    echo "ERROR: $d/CLAUDE.md missing" >&2
    exit 1
  fi
done

# Check 2: docs/frontend.md must NOT exist (replaced by frontend/CLAUDE.md)
if [[ -f docs/frontend.md ]]; then
  echo "ERROR: docs/frontend.md should be removed; its index now lives in frontend/CLAUDE.md" >&2
  exit 1
fi

# Check 3: every docs/frontend/*.md direct child appears in frontend/CLAUDE.md
actual=$(find docs/frontend -maxdepth 1 -type f -name '*.md' -exec basename {} \; | sort -u)
indexed=$(grep -oE 'docs/frontend/[a-z0-9-]+\.md' frontend/CLAUDE.md | sed 's|docs/frontend/||' | sort -u)
diff_out=$(diff -u <(echo "$actual") <(echo "$indexed") 2>/dev/null || true)
if [[ -n "$diff_out" ]]; then
  echo "ERROR: frontend/CLAUDE.md index is out of sync with docs/frontend/ contents" >&2
  echo "--- docs/frontend/ (actual) vs frontend/CLAUDE.md (indexed) ---" >&2
  echo "$diff_out" >&2
  exit 1
fi

# Check 4: every relative Markdown link inside the four area CLAUDE.md files resolves
link_errors=0
for f in backend/CLAUDE.md frontend/CLAUDE.md schema/CLAUDE.md docs/CLAUDE.md; do
  dir=$(dirname "$f")
  # Extract the URL portion of every Markdown link [text](url). Skip http(s) and pure anchor (#...) targets.
  # Use process substitution to avoid subshell exit-code issues under pipefail.
  while IFS= read -r link; do
    [[ "$link" =~ ^https?:// ]] && continue
    [[ "$link" =~ ^# ]] && continue
    # Strip anchor (#section) from the link before resolving.
    path="${link%%#*}"
    target="$dir/$path"
    if [[ ! -f "$target" ]]; then
      echo "ERROR: broken link in $f: $link (resolved to $target)" >&2
      link_errors=$((link_errors + 1))
    fi
  done < <(grep -oE '\]\(([^)]+)\)' "$f" 2>/dev/null | sed -E 's/^\]\(([^)]+)\)$/\1/' || true)
done
if [[ $link_errors -ne 0 ]]; then
  exit 1
fi

echo "OK: CLAUDE.md hierarchy verified"
