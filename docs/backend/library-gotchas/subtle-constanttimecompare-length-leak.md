# `subtle.ConstantTimeCompare` leaks token length — pair with a rate limiter

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

`crypto/subtle.ConstantTimeCompare` returns early (0) when the two byte slices differ in length, leaking length via timing. For equal-length inputs the comparison runs in constant time. The practical impact for bearer-token checking is small when the token length is public knowledge (e.g. a fixed 64-hex-char token), but the leak becomes meaningful for variable-length or secret-length tokens without an external mitigation.

The `/internal/ping` handler pairs `ConstantTimeCompare` with a per-IP rate limiter (1 req/s, burst 5). The rate limiter makes the length-oracle non-exploitable in practice: an attacker cannot iterate quickly enough to extract useful information before being throttled. Any future endpoint that adopts bearer-token comparison **without** a rate limiter must also add one — or switch to a constant-time scheme that does not branch on length.

## Alternative: pre-hash both inputs to a fixed-width digest

A second valid mitigation is to pass both inputs through `sha256.Sum256` *before* `ConstantTimeCompare`. The comparison then runs on two `[32]byte` digests, so the input length is no longer observable through the early-return channel — the timing channel is closed off at the type level rather than in policy. The `/internal/notion-sync` handler uses this pattern:

```go
func validBearer(authHeader, expected string) bool {
    got, ok := strings.CutPrefix(authHeader, "Bearer ")
    if !ok {
        return false
    }
    gotHash := sha256.Sum256([]byte(got))
    expectedHash := sha256.Sum256([]byte(expected))
    return subtle.ConstantTimeCompare(gotHash[:], expectedHash[:]) == 1
}
```

The two mitigations are complementary, not redundant. Pre-hashing closes the length channel at the comparison boundary; the rate limiter caps the attacker's iteration budget. An endpoint that handles a long-lived secret token (i.e. one a brute-force attacker would still want to guess byte-by-byte) should still pair pre-hashing with a rate limiter, because pre-hash alone does not bound how many attempts the attacker can make per second.
