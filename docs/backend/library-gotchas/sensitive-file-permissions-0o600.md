# Sensitive file output: use `0o600`, not `0o644`, for files containing PII

> Part of the [Go library gotchas](./../../../.claude/rules/go-library-gotchas.md) rules.

## Rule

Files written by CLI tools that contain PII must use permission mode `0o600` (owner-read-write only). Using `0o644` (owner-read-write, group-read, other-read) makes the file readable by any process or user on the same system.

```go
// CORRECT
if err := os.WriteFile(outPath, data, 0o600); err != nil {
    return eris.Wrapf(err, "write output file %s", outPath)
}

// WRONG — world-readable
if err := os.WriteFile(outPath, data, 0o644); err != nil { ... }
```

## What counts as PII in this codebase

- Email addresses (from `auth.users` or any user-facing table).
- `auth.users` UUIDs (they are stable identifiers that can be correlated across data sets).
- Dump files produced by the seed CLI (they combine both of the above).
- API tokens, secret keys, or any credential value written to disk.

## Why `0o644` is a real risk

On a shared CI runner or a developer machine with multiple accounts, a file written with `0o644` is readable by every other user on the system. Even on a single-user machine, `0o644` is world-readable by any process that can traverse the parent directory. A dump file that contains user emails sitting in `/tmp` or a project subdirectory with `0o644` violates the Supabase RLS intent — data that the database protects behind row-level policies is exposed in plaintext on the filesystem.

## Creating parent directories

When the output path may not exist yet, create the parent directory with `0o700` before writing the file:

```go
if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
    return eris.Wrapf(err, "create output directory")
}
if err := os.WriteFile(outPath, data, 0o600); err != nil {
    return eris.Wrapf(err, "write output file %s", outPath)
}
```

Using `0o700` for the directory (owner-execute only) prevents other users from listing directory contents even if they can guess the path.
