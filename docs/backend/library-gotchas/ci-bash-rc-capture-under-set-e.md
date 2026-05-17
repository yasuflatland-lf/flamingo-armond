# CI bash `rc=$?` is dead code under `set -e` — use `cmd || rc=$?`

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

GitHub Actions runs `run:` blocks under the default shell `bash --noprofile --norc -eo pipefail {0}`. The `-e` flag aborts the script on the first non-zero exit, *before* the next line runs. A common pattern lifted from non-CI shell scripts therefore behaves wrong in CI:

```bash
# BROKEN under `set -e`: the script never reaches the `if`.
go run ./cmd/schema-lint
rc=$?
if [ "$rc" -ne 0 ]; then
  echo "::error::lint failed with exit $rc"
  exit "$rc"
fi
```

When `go run ./cmd/schema-lint` exits non-zero, `set -e` aborts at that line. The next two lines never execute, `rc` is never assigned, and the job fails without the `::error::` annotation the maintainer expected to surface in the CI summary. Worse, if a later refactor inserts a guaranteed-success command between the call and `rc=$?`, the script silently swallows the failure exit code.

**The fix is the tested-compound form `cmd || rc=$?`.** `set -e` is documented to *not* abort when the failing command is the left-hand side of a `||` (or `&&`, or the condition of `if`/`while`/`until`, or a member of a pipeline that is not the last). The branch is reached only when `cmd` fails, so initialize `rc=0` first:

```bash
# CORRECT: `set -e` skips the abort because `go run` is the LHS of `||`.
rc=0
go run ./cmd/schema-lint || rc=$?
if [ "$rc" -ne 0 ]; then
  echo "::error::lint failed with exit $rc"
  exit "$rc"
fi
```

**How to apply:** any CI `run:` block that wants to inspect a non-zero exit code from a tool — to emit `::error::` annotations, route to different remediation messages by exit code (e.g. exit 1 = lint violation, exit 2 = config error), or chain conditional cleanup — must use the `|| rc=$?` form. The naked `cmd; rc=$?` pattern is a porting hazard from local shell scripts that ran without `set -e`. The same gotcha applies to any `run:` block in `.github/workflows/*.yml` since the default shell flags are inherited workflow-wide unless explicitly overridden by `shell:` or `defaults.run.shell:`.
