# Playbook authoring patterns

This document records non-obvious Ansible idioms and test-infrastructure decisions that apply across all playbooks in `playbooks/`. It exists because these patterns are easy to get wrong on first contact and the mistakes are silent enough that multiple independent reviewers have rediscovered them. `docs/deployment.md` is the operational guide (what to run and when); the playbooks under `playbooks/setup-prod/` and `playbooks/teardown-prod/` are the specific implementations. This document covers the *why* behind structural choices that both share.

## Ansible idioms

### `psql` stdout vs stderr: `RETURNING` corrupts `changed_when` detection

`psql` writes the command tag (`INSERT 0 1`) to **stdout**, not stderr. When a task
uses `register:` on a `command:` / `shell:` that runs `psql`, the tag appears in
`result.stdout`, and a `changed_when: "'INSERT 0 1' in result.stdout"` check works
correctly.

Adding `RETURNING <cols>` to the SQL mixes row-set data into stdout alongside the
command tag. Because the tag is no longer the only line on stdout, a naive
`'INSERT 0 1' in result.stdout` check may still match — but the presence of extra
rows makes the detection fragile for multi-row inserts where the tag shifts to
`INSERT 0 N`. If the `changed_when` goal is simply "did a new row land", drop
`RETURNING` entirely and check for `INSERT 0 1` in stdout; the row-set output is
not needed when the task only needs to detect whether the insert fired.

**How to apply:** when writing an Ansible task that wraps a `psql` insert, choose
one of two shapes:

- **No `RETURNING`, check stdout for tag** — reliable and sufficient when you only
  need to know whether the row was inserted.
- **Keep `RETURNING`, parse stdout structurally** — only if the task must consume
  the returned columns; in this case do not also rely on `INSERT 0 1` detection
  because the stdout line order varies.

### `supabase db remote sql` does not exist — use `psql` directly

`supabase db remote` is a schema-diff tool; it does not accept arbitrary SQL.
Attempting `supabase db remote sql` returns an error. To run a SQL file or
heredoc against a remote Supabase project from an Ansible task, extract the
connection string and call `psql` directly:

```yaml
- name: Get DB_URL from supabase status
  command: >
    python3 -c "
    import subprocess, json
    out = subprocess.check_output(['supabase', 'status', '--output', 'json'])
    print(json.loads(out)['DB URL'])
    "
  register: db_url_result

- name: Run admin SQL
  command: >
    psql "{{ db_url_result.stdout | trim }}" -c "{{ sql_statement }}"
  register: psql_result
  changed_when: "'INSERT 0 1' in psql_result.stdout"
```

Using `supabase status --output json` (parsed with Python) rather than the shell
one-liner `supabase status -o env | grep '^DB_URL='` is more robust to quoting and
to future changes in the text-format output. Both forms appear in `docs/dev-setup.md`
§ "Manual SQL fallback" for human use; the JSON form is preferred inside Ansible.

**Why:** a `supabase db remote sql` call that compiles cleanly in a task file will
fail at runtime with an unhelpful "unknown command" error. The fix is not a flag
change — the subcommand simply does not exist. Always verify CLI subcommands against
`supabase --help` / `supabase <cmd> --help` before authoring Ansible tasks.

### `block: when:` is the correct early-exit idiom inside `include_tasks`

`meta: end_host` and `meta: end_play` both terminate scope at the play level — not at the included file level. Calling either inside an `include_tasks` fragment causes the *entire play* (all phases) to stop, not just the current fragment. The correct idiom for a conditional short-circuit (e.g. a 404 means "already done") is to wrap the remaining tasks inside a `block:` guarded by `when: <condition>`. Both the 404 block and the 200 block fall through to the end of the file and the calling play continues normally. Reaching for `end_host` or `end_play` is natural but wrong here.

### `rescue` is a reserved YAML keyword in Ansible task context

A top-level play variable named `rescue` produces an Ansible WARNING because the same identifier is the error-handling clause of the `block/rescue/always` construct. The warning reads `[WARNING]: Skipping unexpected key in …: rescue`. This is currently non-fatal, but Ansible reserves the right to make it an error in a future version. Use `rescue_mode` or another non-reserved name for any boolean flag that enables advisory/fallback behavior.

### `default(omit)` inside `set_fact` dict literals has undefined behavior

`default(omit)` is designed for module *argument* processing: when a variable evaluates to `omit`, Ansible removes the corresponding argument from the module call. That contract does not extend to dict values constructed inside `set_fact`. In some Ansible versions, using `default(omit)` as a dict value silently drops the key; in others it retains `None`; in others the behavior depends on whether the source variable was undefined vs. empty. To keep behavior predictable across Ansible versions, use `default(None)` inside `set_fact` dict literals so the key is always present with a `null` value.

### Single source-of-truth `set_fact` for dual-destination secrets

When the same secret must be written identically to two or more destinations (e.g. a server env-var store and a CI provider secret store), calling `lookup('env', ...)` independently in each writer is structurally fragile: an env-var change between task executions, or operator confusion about which source is canonical, causes the destinations to drift silently. Capture the value once into a `set_fact` early in the play, mark `no_log: true`, and reference that fact in every writer. This makes the "same value" invariant structural rather than convention-based.

```yaml
- name: Capture NOTION_SYNC_TOKEN for dual-destination write
  ansible.builtin.set_fact:
    notion_sync_token: "{{ lookup('env', 'NOTION_SYNC_TOKEN') }}"
  no_log: true
  tags: [notion, notion-secrets, postapply]
```

Use `value: "{{ notion_sync_token }}"` in both the Render env-var loop and the `gh secret set` task. A single capture point guarantees bearer-auth integrity across both consumers.

### `[<tag>, never]` keeps a block out of default runs but `--tags <tag>` opts in

The `never` tag tells Ansible to skip the task on any tag-less invocation. Pairing `never` with a feature tag — e.g. `tags: [notion-local, never]` — keeps the block out of `make setup` / `make sync-env` (which run with no `--tags`), while an explicit `make notion-local-setup` (which passes `--tags notion-local`) still reaches it. This is the right shape for an opt-in flow that shares a play with the default flow but must not run by default. Without `never`, Ansible runs every task on a tag-less invocation regardless of `tags:`. See `playbooks/setup.yml` for the notion-local block; the same pattern applies to seed-admin (`tags: [seed-admin, never]`).

### `psql -v name=value` + `:'name'` for parameterized SQL

Building SQL by interpolating user-controlled values (e.g. an email from `.env`) into the query string is unsafe even when the value is escaped, because every escape layer (Jinja → shell → psql) has its own rules and the composition is fragile. `psql` accepts named variables via `-v name=value` and substitutes them with `:'name'` (single-quoted, escaped) inside the SQL — structurally analogous to a prepared statement.

```yaml
- name: Look up auth.users.id by email (parameterized)
  ansible.builtin.command:
    argv:
      - psql
      - "{{ db_url }}"
      - -v
      - "ON_ERROR_STOP=1"
      - -v
      - "email={{ email_value | trim }}"
      - -t
      - -A
      - -c
      - "SELECT id FROM auth.users WHERE lower(email) = lower(:'email')"
```

Any upstream regex / format validation on the email is an early-fail UX guard, not a security guard — the `:'email'` substitution is what keeps the call safe regardless of what made it past the validator.

### Accumulator preflight for multi-key validation

A per-iteration `ansible.builtin.fail` inside a preflight loop reports only the FIRST missing env var, forcing the operator into a fix-and-rerun cycle for each key. Prefer the accumulator pattern: collect all missing keys into a `set_fact` list and fail once with the full set. Reset the accumulator before the loop to ensure idempotency on re-entry.

```yaml
- name: Reset missing-key accumulator
  ansible.builtin.set_fact:
    _missing_keys: []

- name: Collect missing keys
  ansible.builtin.set_fact:
    _missing_keys: "{{ _missing_keys | default([]) + [item] }}"
  when: (lookup('env', item) | default('')) | trim | length == 0
  loop: [KEY_A, KEY_B, KEY_C]

- name: Fail if any required keys are missing
  ansible.builtin.fail:
    msg: "Missing required env vars: {{ _missing_keys | join(', ') }}"
  when: (_missing_keys | default([])) | length > 0
```

Net cost: +1 task over the per-iteration form. Net benefit: fresh-setup operators see the full missing set in one cycle.

### Lexicographic ISO 8601 enables string-based archive enumeration

Timestamps written in `YYYY-MM-DDTHH:MM:SSZ` format (fixed-width, sub-seconds truncated, always suffixed `Z`) are lexicographically sortable. String comparison (`>=`) produces the same order as chronological comparison, which means archive files can be enumerated and filtered by run using a plain Jinja string test rather than a date parser. The `postapply.yml` phase uses this property to collect all archive files belonging to the current run by comparing each file's embedded `teardown_at` against the `teardown_started_at` fact set at the beginning of preflight. Deviating from this format (e.g. sub-second precision, offset notation) breaks the string-ordering invariant.

## Ansible tag and fact hygiene

The patterns in this section are pitfalls — structural mistakes that produce silent wrong behavior rather than loud failures. Each was discovered during the Notion-sync wiring.

### Tag inheritance silently drops on first explicit `tags:` add

A task with no `tags:` line inherits ALL invocation tags: it runs under any `--tags X`. Adding `tags: [notion]` REPLACES that inheritance — the task now ONLY runs under `--tags notion`, silently dropping reachability under `--tags postapply`. This caused cascading regressions during Notion-sync implementation: a previously-untagged Render env reconciliation block was scoped to `[notion, notion-secrets]`, breaking `make setup-prod-postapply`.

**Rule:** when adding tags to a task that previously had none, also include every tag that previously reached it via inheritance — typically the phase tag (`postapply`, `teardown`, etc.) for the file.

### Producer tag-set must contain the union of all consumer tag-sets

If a `set_fact` task carries tags `[A, B]` but a downstream task that references `{{ that_fact }}` carries tags `[A, C]`, then running `--tags C` reaches the consumer with the fact undefined. This causes a Jinja2 evaluation failure — or worse, a silent empty-string write when the consuming task has a `default('')` guard. The same applies to preflight tasks: if the preflight tags don't cover every entry path that reaches a writer, missing-required-value checks silently no-op.

**Rule:** every preflight task and `set_fact` producer must carry the UNION of every downstream consumer's tag set. For Notion sync, that union is `[preflight, notion-preflight, notion, notion-secrets, postapply]` on all three preflight tasks. The same union rule applies to ownership-detection set_facts shared across flows: when `notion-local` reads `env_ownership[...]` to decide whether to skip a managed file, the upstream stat / slurp / set_fact tasks that build `env_ownership` must carry `[sync-env, notion-local]`, or the consumer evaluates an undefined dict and fails (or worse, silently no-ops behind a `default({})`).

### `when: (item.value | length) > 0` masks silent empty-write no-ops

A loop that writes env vars and guards with `when: length > 0` to skip empty values seems defensive, but in combination with a preflight whose tag set does not cover all entry paths (see above), missing required values become silent no-ops instead of fast failures. Every `when: length > 0` guard in a writer must be paired with a preflight whose tag set covers every entry path that reaches that writer.

### `debug` for a should-halt condition is invisible at default verbosity

`make` invokes `ansible-playbook` without `-v`, so `debug:` task output is suppressed unless an operator passes `-vv` manually. Using `debug: msg: "skipping because …"` to announce a condition that should actually stop the play is a silent regression vector: the play continues past the "skip" with downstream tasks operating on undefined or stale facts, and the operator sees no warning. For any condition that should halt the flow (missing required file, ownership marker absent, ambiguous resolution result), use `ansible.builtin.fail:` with an actionable `msg:`. Reserve `debug:` for diagnostic values an operator would only consult under `-vv`.

### `lineinfile: create: false` is a silent no-op when the target is missing

`ansible.builtin.lineinfile` with `create: false` does nothing when the target file does not exist — no error, no warning, just `ok` in the play recap. This is the desired behavior when intentionally refusing to create a file (e.g. when the parent flow expects an upstream task to seed it), but it requires an upstream existence guard to convert "file missing" from a silent skip into an explicit fail. The notion-local block guards with two upstream `fail:` tasks before the writer loop: one for `env_ownership[...] == 'missing'` and one for `'user-owned'`. Without those guards, a fresh checkout that has not run `make sync-env` yet would print "wrote NOTION_* keys" while writing nothing.

### `gh secret set` always reports `changed_when: rc == 0`

GitHub's `gh secret set` API has no diff semantic — it always overwrites and returns `0` on success. A `changed_when: _result.rc == 0` guard therefore reports `changed` on every successful run, even when the value is identical. This is consistent with the existing `PING_TOKEN` / `RENDER_PING_URL` convention in this repo, but operators reviewing playbook output should not interpret repeated `changed` as evidence of actual state change; it is an API limitation, not a drift indicator.

## Test infrastructure

### Ansible's `uri` module requires a real HTTP server for integration tests

The `uri` module uses Python's `urllib`/`urllib3` stack, not the `requests` library. Mocking libraries that patch `requests` (such as `responses`) have no effect on `uri` module HTTP calls issued from an `ansible-playbook` subprocess. The only reliable way to intercept those calls in integration tests is to bind a real local socket — `pytest-httpserver` (a `werkzeug`-based fixture) is the standard choice. Unit-level checks that exercise pure Python or Jinja logic can stay in-process and skip the server entirely.

### The `^test_` prefix makes test mode structurally unable to touch production

`verify_identity.yml` asserts `not (testing | default(false)) or resource_id is match('^test_.*')` before issuing any destructive call. When `testing=true`, any resource id that does not begin with `test_` fails the assertion and aborts the phase with a clear message. Production runs set `testing=false` (or omit it), so the prefix check never fires. The value of this pattern is that it cannot be bypassed by accident: a CI run that accidentally receives a real production resource id while `testing=true` is set will abort before the DELETE, not after.

### Idempotent secret generation: guard before generating, not after

When a phase auto-generates a secret (e.g. `openssl rand -hex 32` for `PING_TOKEN`), wrap the generation in a `when:` guard so that re-runs preserve an already-generated token rather than rotating it unexpectedly:

```yaml
- name: Generate PING_TOKEN if not present
  set_fact:
    ping_token: "{{ lookup('pipe', 'openssl rand -hex 32') }}"
  when: ping_token is not defined or (ping_token | string | length) == 0
```

The guard checks both `is not defined` (first run, key absent from state file) and `length == 0` (key present but empty, e.g. from a corrupted state file). Without this guard, every `make setup-prod` re-run would rotate `PING_TOKEN`, requiring a simultaneous update of the GitHub secret, the Render env var, and any other consumer — defeating the purpose of automation.

## Recovering from a dirty migration

### Symptom

The application fails to boot with a log line similar to:

```
Dirty database version 20260430080000. Fix and force version.
```

The Render deploy status stalls at `update_failed`, and re-running `make setup-prod-postapply` returns the same error on every subsequent attempt.

### Why it happens

golang-migrate commits `dirty = true` in `public.schema_migrations` in a dedicated transaction BEFORE it executes the migration SQL. If the migration SQL then fails or rolls back, the dirty record is already committed and remains. The pgx/v5 driver does not auto-wrap migration files in a transaction, so any DDL that succeeded before the failure is also committed independently. A single migration file that mixes DDL (`CREATE TABLE`) and privilege-sensitive `ALTER TABLE` statements (e.g. `ENABLE ROW LEVEL SECURITY`) can therefore partially succeed — some statements commit, others fail — leaving the schema in an inconsistent state with `dirty = true`.

Subsequent deploy attempts only report the dirty error; the original SQL error that caused the partial failure appears only in the first failing deploy's log.

### Diagnose

Connect to the database and run:

```sql
SELECT version, dirty FROM public.schema_migrations;
```

If `dirty = true`, locate the first failing deploy's log (in the Render dashboard or equivalent). Retries only show the dirty guard error; the root-cause SQL error is in the first log entry.

### Recover: forward-fix (preferred)

Apply the missing statements manually, then clear the dirty flag:

1. Read the first failing deploy log to find the exact SQL error.
2. Apply the missing DDL via the database SQL editor (e.g. the Supabase SQL editor or `psql`).
3. Clear the dirty flag:

   ```sql
   UPDATE public.schema_migrations SET dirty = false WHERE version = <N>;
   ```

4. Re-run `make setup-prod-postapply`.

### Recover: backward-fix (last resort)

Use this only when the forward-fix is not safe (e.g. the partial migration left data in an inconsistent state that cannot be resolved without a rollback):

1. Manually undo any partial schema changes.
2. Roll the version pointer back and clear the dirty flag:

   ```sql
   UPDATE public.schema_migrations SET dirty = false, version = <previous>;
   ```

   WARNING: this is rare and risky. It tells golang-migrate that the previous migration was the last clean state, so the next deploy will attempt to re-apply the failed migration from scratch.

3. Re-deploy.

### Prevention

Two structural changes reduce the blast radius of future migration failures:

- RLS and other privilege-sensitive `ALTER TABLE` statements are split into their own migration file, isolated from the DDL that creates the tables. A failure in one file does not affect the other.
- Each migration file is wrapped in an explicit `BEGIN; ... COMMIT;` block so that all statements in the file succeed or fail atomically.

## Cross-references

The following operational expressions of the above patterns are documented in `docs/deployment.md`:

- **Block/rescue around DELETE for fail-archive** — the teardown playbook wraps each provider DELETE in `block/rescue` so that a 5xx retry exhaustion writes a `delete_status: failed` archive entry instead of silently losing the audit trail. See `docs/deployment.md` § "When a DELETE exhausts retries on 5xx".
- **404 idempotency via dual-block pattern** — the pre-GET + dual-block structure means re-running a partially complete teardown is always safe: already-deleted resources produce `delete_status: already_gone` entries and the play continues. See `docs/deployment.md` § "Recovery".
- **State-file lifecycle** — the state file is loaded once in preflight, embedded in the first archive of the run, and deleted only in postapply. It is preserved through partial failures. See `docs/deployment.md` § "State file" and § "Why teardown is irreversible".
