# GORM v1 string-typed primary key with DB-generated UUID requires `default:` tag

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

GORM's `BeforeCreate` hook auto-generates a UUID when a primary key field is a `string` type and is zero-valued — but only when the field carries `gorm:"default:..."` in its tag. Without the tag, GORM leaves the field empty and the INSERT fails.

Peer models (`gormUser`, `gormCard`) avoid this by supplying the UUID in the application layer before calling `Create`. `gormPingRecord` is different: `Create` inserts a row without any caller-supplied ID, so the DB must generate it via `gen_random_uuid()`. The tag `gorm:"default:gen_random_uuid()"` is therefore load-bearing even though `AutoMigrate` is not used and the column default is already defined in the migration SQL.
