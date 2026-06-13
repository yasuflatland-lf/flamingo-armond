package domain

import "testing"

func TestParseLearnDisplayMode(t *testing.T) {
	cases := []struct {
		in      string
		want    LearnDisplayMode
		wantErr bool
	}{
		{"flip_to_reveal", LearnDisplayFlipToReveal, false},
		{"always_visible", LearnDisplayAlwaysVisible, false},
		{"  flip_to_reveal  ", LearnDisplayFlipToReveal, false}, // trims
		{"", "", true},
		{"bogus", "", true},
	}
	for _, c := range cases {
		got, err := ParseLearnDisplayMode(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("ParseLearnDisplayMode(%q): want error, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseLearnDisplayMode(%q): unexpected error %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("ParseLearnDisplayMode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDefaultLearnDisplayMode(t *testing.T) {
	if DefaultLearnDisplayMode != LearnDisplayFlipToReveal {
		t.Fatalf("default = %q, want flip_to_reveal", DefaultLearnDisplayMode)
	}
}
