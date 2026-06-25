package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestParseCoverImageURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "https ok", input: "https://example.com/cover.png", want: "https://example.com/cover.png"},
		{name: "http ok", input: "http://example.com", want: "http://example.com"},
		{name: "trims whitespace", input: "  https://example.com/a.jpg  ", want: "https://example.com/a.jpg"},
		{name: "javascript scheme rejected", input: "javascript:alert(1)", wantErr: ErrCoverImageURLInvalid},
		{name: "data scheme rejected", input: "data:image/png;base64,AAAA", wantErr: ErrCoverImageURLInvalid},
		{name: "ftp scheme rejected", input: "ftp://example.com/a.png", wantErr: ErrCoverImageURLInvalid},
		{name: "scheme-less rejected", input: "example.com/a.png", wantErr: ErrCoverImageURLInvalid},
		{name: "missing host rejected", input: "https://", wantErr: ErrCoverImageURLInvalid},
		{name: "too long rejected", input: "https://example.com/" + strings.Repeat("a", CoverImageURLMax), wantErr: ErrCoverImageURLTooLong},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseCoverImageURL(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
