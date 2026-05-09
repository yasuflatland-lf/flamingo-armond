# `echo.NewHTTPError` discards manually-set response headers

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Echo's default `HTTPErrorHandler` serializes the error and writes a fresh response, discarding any headers set on the context before returning the error. Setting `c.Response().Header().Set("WWW-Authenticate", "...")` and then `return echo.NewHTTPError(401, "...")` will drop the header in the rendered response. The fix is to write the response directly and return `nil`:

```go
c.Response().Header().Set("WWW-Authenticate", `Bearer realm="api"`)
return c.String(http.StatusUnauthorized, "unauthorized")
```

Any future middleware that must send headers on an error response must use this pattern.
