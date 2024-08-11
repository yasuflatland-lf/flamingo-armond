package utils_test

import (
	"backend/pkg/utils"
	"testing"
)

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		target   string
		compare  string
		expected int64
	}{
		// Test case where target and compare are exactly the same
		{"kitten", "kitten", 0},

		// Test case where target and compare differ by one character
		{"kitten", "sitten", 1},

		// Test case where target and compare have no matching characters
		{"abc", "xyz", 3},

		// Test case where target and compare are 300 characters long and completely different
		{"いろはにほへとちりぬるをわかよたれそつねならむうゐのおくやまけふこえてあさきゆめみしゑひもせす", "いろはにほへとちりぬるをわかよたれそつねならむうゐのおうやまけふこえてあさきゆめみしゑひもせす", 1},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.target+" vs "+tt.compare, func(t *testing.T) {
			t.Parallel() // Run tests in parallel
			result := utils.LevenshteinDistance(tt.target, tt.compare)
			if result != tt.expected {
				t.Errorf("LevenshteinDistance(%q, %q) = %d; want %d", tt.target, tt.compare, result, tt.expected)
			}
		})
	}
}

func TestSimilarity(t *testing.T) {
	tests := []struct {
		target   string
		compare  string
		expected float64
		epsilon  float64
	}{
		{"kitten", "kitten", 1.0, 1e-9},
		{"kitten", "sitten", 5.0 / 6.0, 1e-9},
		{"abc", "xyz", 0.0, 1e-9},
		{"いろはにほへとちりぬるをわかよたれそつねならむうゐのおくやまけふこえてあさきゆめみしゑひもせす", "いろはにほへとちりぬるをわかよたれそつねならむうゐのおうやまけふこえてあさきゆめみしゑひもせす", 0.9929078014, 1e-9},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.target+" vs "+tt.compare, func(t *testing.T) {
			t.Parallel() // Run tests in parallel
			result := utils.Similarity(tt.target, tt.compare)
			if !utils.Float64Equal(result, tt.expected, tt.epsilon) {
				t.Errorf("Similarity(%q, %q) = %.10f; want %.10f", tt.target, tt.compare, result, tt.expected)
			}
		})
	}
}
