# `subtle.ConstantTimeCompare` leaks token length — pair with a rate limiter

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

`crypto/subtle.ConstantTimeCompare` returns early (0) when the two byte slices differ in length, leaking length via timing. For equal-length inputs the comparison runs in constant time. The practical impact for bearer-token checking is small when the token length is public knowledge (e.g. a fixed 64-hex-char token), but the leak becomes meaningful for variable-length or secret-length tokens without an external mitigation.

The `/internal/ping` handler pairs `ConstantTimeCompare` with a per-IP rate limiter (1 req/s, burst 5). The rate limiter makes the length-oracle non-exploitable in practice: an attacker cannot iterate quickly enough to extract useful information before being throttled. Any future endpoint that adopts bearer-token comparison **without** a rate limiter must also add one — or switch to a constant-time scheme that does not branch on length.
