# Playbook authoring patterns

This document records non-obvious Ansible idioms and test-infrastructure decisions that apply across all playbooks in `playbooks/`. It exists because these patterns are easy to get wrong on first contact and the mistakes are silent enough that multiple independent reviewers have rediscovered them. `docs/deployment.md` is the operational guide (what to run and when); the playbooks under `playbooks/setup-prod/` and `playbooks/teardown-prod/` are the specific implementations. This document covers the *why* behind structural choices that both share.

## Ansible idioms

### `block: when:` is the correct early-exit idiom inside `include_tasks`

`meta: end_host` and `meta: end_play` both terminate scope at the play level — not at the included file level. Calling either inside an `include_tasks` fragment causes the *entire play* (all phases) to stop, not just the current fragment. The correct idiom for a conditional short-circuit (e.g. a 404 means "already done") is to wrap the remaining tasks inside a `block:` guarded by `when: <condition>`. Both the 404 block and the 200 block fall through to the end of the file and the calling play continues normally. Reaching for `end_host` or `end_play` is natural but wrong here.

### `rescue` is a reserved YAML keyword in Ansible task context

A top-level play variable named `rescue` produces an Ansible WARNING because the same identifier is the error-handling clause of the `block/rescue/always` construct. The warning reads `[WARNING]: Skipping unexpected key in …: rescue`. This is currently non-fatal, but Ansible reserves the right to make it an error in a future version. Use `rescue_mode` or another non-reserved name for any boolean flag that enables advisory/fallback behavior.

### `default(omit)` inside `set_fact` dict literals has undefined behavior

`default(omit)` is designed for module *argument* processing: when a variable evaluates to `omit`, Ansible removes the corresponding argument from the module call. That contract does not extend to dict values constructed inside `set_fact`. In some Ansible versions, using `default(omit)` as a dict value silently drops the key; in others it retains `None`; in others the behavior depends on whether the source variable was undefined vs. empty. To keep behavior predictable across Ansible versions, use `default(None)` inside `set_fact` dict literals so the key is always present with a `null` value.

### Lexicographic ISO 8601 enables string-based archive enumeration

Timestamps written in `YYYY-MM-DDTHH:MM:SSZ` format (fixed-width, sub-seconds truncated, always suffixed `Z`) are lexicographically sortable. String comparison (`>=`) produces the same order as chronological comparison, which means archive files can be enumerated and filtered by run using a plain Jinja string test rather than a date parser. The `postapply.yml` phase uses this property to collect all archive files belonging to the current run by comparing each file's embedded `teardown_at` against the `teardown_started_at` fact set at the beginning of preflight. Deviating from this format (e.g. sub-second precision, offset notation) breaks the string-ordering invariant.

## Test infrastructure

### Ansible's `uri` module requires a real HTTP server for integration tests

The `uri` module uses Python's `urllib`/`urllib3` stack, not the `requests` library. Mocking libraries that patch `requests` (such as `responses`) have no effect on `uri` module HTTP calls issued from an `ansible-playbook` subprocess. The only reliable way to intercept those calls in integration tests is to bind a real local socket. The test harness under `playbooks/test/` uses `pytest-httpserver` (a `werkzeug`-based fixture) for exactly this reason. Unit-level tests that exercise pure Python or Jinja logic do not need the server — see `test_archive_shape.py`, `test_dep_map.py`, and `test_name_match.py` for examples of the purely in-process tier.

### The `^test_` prefix makes test mode structurally unable to touch production

`verify_identity.yml` asserts `not (testing | default(false)) or resource_id is match('^test_.*')` before issuing any destructive call. When `testing=true`, any resource id that does not begin with `test_` fails the assertion and aborts the phase with a clear message. Production runs set `testing=false` (or omit it), so the prefix check never fires. The value of this pattern is that it cannot be bypassed by accident: a CI run that accidentally receives a real production resource id while `testing=true` is set will abort before the DELETE, not after.

## Cross-references

The following operational expressions of the above patterns are documented in `docs/deployment.md`:

- **Block/rescue around DELETE for fail-archive** — the teardown playbook wraps each provider DELETE in `block/rescue` so that a 5xx retry exhaustion writes a `delete_status: failed` archive entry instead of silently losing the audit trail. See `docs/deployment.md` § "When a DELETE exhausts retries on 5xx".
- **404 idempotency via dual-block pattern** — the pre-GET + dual-block structure means re-running a partially complete teardown is always safe: already-deleted resources produce `delete_status: already_gone` entries and the play continues. See `docs/deployment.md` § "Recovery".
- **State-file lifecycle** — the state file is loaded once in preflight, embedded in the first archive of the run, and deleted only in postapply. It is preserved through partial failures. See `docs/deployment.md` § "State file" and § "Why teardown is irreversible".
