package repository

import "testing"

func TestEscapeLikePattern(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text unchanged",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "percent sign escaped",
			input: "50%",
			want:  `50\%`,
		},
		{
			name:  "underscore escaped",
			input: "foo_bar",
			want:  `foo\_bar`,
		},
		{
			name:  "backslash escaped first",
			input: `a\b`,
			want:  `a\\b`,
		},
		{
			name:  "all three meta-characters mixed",
			input: `50% off_sale\deal`,
			want:  `50\% off\_sale\\deal`,
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeLikePattern(tc.input)
			if got != tc.want {
				t.Errorf("escapeLikePattern(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}
