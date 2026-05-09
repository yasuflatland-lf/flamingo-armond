package notion

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/rotisserie/eris"
)

var (
	ErrRetryElapsed  = errors.New("notion: retry-after max elapsed exceeded")
	ErrRetryAttempts = errors.New("notion: retry attempts exceeded")
)

type RetryConfig struct {
	MaxAttempts int
	MaxElapsed  time.Duration
	Transport   http.RoundTripper
	Logger      *slog.Logger
	Sleep       func(context.Context, time.Duration) error
}

func NewHTTPClient(cfg RetryConfig) *http.Client {
	return &http.Client{Transport: NewRetryAfterRoundTripper(cfg)}
}

func NewRetryAfterRoundTripper(cfg RetryConfig) http.RoundTripper {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.MaxElapsed <= 0 {
		cfg.MaxElapsed = 2 * time.Minute
	}
	if cfg.Transport == nil {
		cfg.Transport = http.DefaultTransport
	}
	if cfg.Sleep == nil {
		cfg.Sleep = sleepContext
	}
	return &retryAfterRoundTripper{cfg: cfg}
}

type retryAfterRoundTripper struct {
	cfg RetryConfig
}

func (rt *retryAfterRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	for attempt := 1; ; attempt++ {
		res, err := rt.cfg.Transport.RoundTrip(req)
		if err != nil {
			return nil, eris.Wrap(err, "notion: round trip")
		}
		if !shouldRetryStatus(res.StatusCode) {
			return res, nil
		}
		if attempt >= rt.cfg.MaxAttempts {
			discardAndClose(res.Body)
			return nil, eris.Wrap(ErrRetryAttempts, "notion: retry request")
		}

		wait := retryDelay(res, attempt)
		if rt.cfg.Logger != nil {
			rt.cfg.Logger.WarnContext(req.Context(), "notion: retrying request",
				"attempt", attempt,
				"notion_status", res.StatusCode,
				"retry_after_s", wait.Seconds(),
			)
		}
		discardAndClose(res.Body)

		if time.Since(start)+wait > rt.cfg.MaxElapsed {
			return nil, eris.Wrap(ErrRetryElapsed, "notion: retry request")
		}
		if err := rt.cfg.Sleep(req.Context(), wait); err != nil {
			return nil, eris.Wrap(err, "notion: sleep interrupted")
		}
	}
}

func shouldRetryStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func retryDelay(res *http.Response, attempt int) time.Duration {
	if res != nil {
		if h := res.Header.Get("Retry-After"); h != "" {
			if sec, err := strconv.Atoi(h); err == nil && sec >= 0 {
				return time.Duration(sec) * time.Second
			}
			if t, err := http.ParseTime(h); err == nil {
				if d := time.Until(t); d > 0 {
					return d
				}
			}
		}
	}
	if res != nil && res.StatusCode == http.StatusTooManyRequests {
		return time.Second
	}
	d := time.Duration(1<<(attempt-1)) * time.Second
	if d > 5*time.Second {
		return 5 * time.Second
	}
	return d
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func discardAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}
