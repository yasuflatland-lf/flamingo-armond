package domain

import (
	"net/url"
	"strings"

	"github.com/rotisserie/eris"
)

// CoverImageURLMax is the maximum byte length of a master cardgroup cover image
// URL (a conventional URL length cap).
const CoverImageURLMax = 2048

var (
	// ErrCoverImageURLInvalid is returned when the trimmed cover image URL does
	// not parse, lacks a host, or uses a scheme other than http/https.
	ErrCoverImageURLInvalid = eris.New("domain: cover image url is invalid")
	// ErrCoverImageURLTooLong is returned when the trimmed cover image URL
	// exceeds CoverImageURLMax characters.
	ErrCoverImageURLTooLong = eris.Errorf("domain: cover image url exceeds %d characters", CoverImageURLMax)
)

// ParseCoverImageURL trims s and validates it as an absolute http(s) URL with a
// host, no longer than CoverImageURLMax. It returns the trimmed URL string on
// success. Rejecting non-http(s) schemes (e.g. javascript:, data:) closes the
// latent XSS surface should a cover image ever be rendered as an <img src>.
func ParseCoverImageURL(s string) (string, error) {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) > CoverImageURLMax {
		return "", ErrCoverImageURLTooLong
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", ErrCoverImageURLInvalid
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrCoverImageURLInvalid
	}
	if u.Host == "" {
		return "", ErrCoverImageURLInvalid
	}
	return trimmed, nil
}
