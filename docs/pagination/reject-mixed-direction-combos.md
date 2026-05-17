# Reject mixed-direction argument combos at the usecase

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `frontend/CLAUDE.md`.

The repository trusts its inputs. Without explicit usecase-layer guards, malformed combinations silently re-interpret as a forward page-1 request and the client never learns why their cursor was ignored. Reject all five bad combos with `BAD_USER_INPUT` (with `extensions.field` naming the offending argument):

| Combo | Why rejected |
|---|---|
| `after` + `before` | The two cursors disagree on direction. |
| `first` + `before` | `first` implies forward, `before` implies backward. |
| `last` + `after` | `last` implies backward, `after` implies forward. |
| `before` alone (no `last`) | A backward cursor without a backward page size is ambiguous. |
| `after` alone (no `first`) | A forward cursor without a forward page size is ambiguous. |

A request with neither cursor and neither size is the legitimate "first page, server default" case and must still be accepted.
