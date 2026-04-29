package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCardgroupShape(t *testing.T) {
	t.Parallel()

	cgType := reflect.TypeOf(Cardgroup{})
	want := map[string]reflect.Type{
		"ID":        reflect.TypeOf(""),
		"OwnerID":   reflect.TypeOf(""),
		"Name":      reflect.TypeOf(""),
		"CreatedAt": reflect.TypeOf(time.Time{}),
		"UpdatedAt": reflect.TypeOf(time.Time{}),
	}

	for name, typ := range want {
		field, ok := cgType.FieldByName(name)
		if !ok {
			t.Fatalf("Cardgroup missing field %s", name)
		}
		if field.Type != typ {
			t.Fatalf("Cardgroup.%s type = %v, want %v", name, field.Type, typ)
		}
		if len(field.Tag) != 0 {
			t.Fatalf("Cardgroup.%s should not have struct tags, got %q", name, field.Tag)
		}
	}
}

func TestCardgroup_Validate(t *testing.T) {
	t.Parallel()

	// zwjEmoji is a family emoji composed of multiple code points joined by
	// zero-width joiners; uniseg counts it as exactly one grapheme cluster.
	const zwjEmoji = "👨‍👩‍👧‍👦"

	cases := []struct {
		name      string
		input     string
		wantErr   bool
		sentinelErr error
	}{
		{
			name:        "empty string",
			input:       "",
			wantErr:     true,
			sentinelErr: ErrCardgroupNameRequired,
		},
		{
			name:        "whitespace only",
			input:       "   ",
			wantErr:     true,
			sentinelErr: ErrCardgroupNameRequired,
		},
		{
			name:    "single latin char",
			input:   "a",
			wantErr: false,
		},
		{
			name:    "exactly 100 latin chars",
			input:   strings.Repeat("a", 100),
			wantErr: false,
		},
		{
			name:        "101 latin chars",
			input:       strings.Repeat("a", 101),
			wantErr:     true,
			sentinelErr: ErrCardgroupNameTooLong,
		},
		{
			name:    "single ZWJ emoji (one grapheme cluster)",
			input:   zwjEmoji,
			wantErr: false,
		},
		{
			name:    "100 ZWJ emojis (each one grapheme)",
			input:   strings.Repeat(zwjEmoji, 100),
			wantErr: false,
		},
		{
			name:        "101 ZWJ emojis",
			input:       strings.Repeat(zwjEmoji, 101),
			wantErr:     true,
			sentinelErr: ErrCardgroupNameTooLong,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cg := &Cardgroup{Name: tc.input}
			err := cg.Validate()
			if tc.wantErr {
				require.Error(t, err, "expected an error for input %q", tc.input)
				if tc.sentinelErr != nil {
					require.True(t, errors.Is(err, tc.sentinelErr),
						"expected errors.Is(err, %v), got %v", tc.sentinelErr, err)
				}
			} else {
				require.NoError(t, err, "expected no error for input %q", tc.input)
			}
		})
	}
}
