package utils

import (
	"math"
	"os"
	"path/filepath"
)

// GetFullPath takes a relative path and returns the full absolute path
func GetFullPath(path string) (string, error) {
	cleanedPath := filepath.Clean(path)
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(currentDir, cleanedPath)
	return fullPath, nil
}

// LevenshteinDistance calculates the Levenshtein distance between two strings, using int64 for large values
func LevenshteinDistance(target, compare string) int64 {
	targetLen := int64(len(target))
	compareLen := int64(len(compare))

	// Initialize a 2D slice (matrix) with int64 values
	dp := make([][]int64, targetLen+1)
	for i := range dp {
		dp[i] = make([]int64, compareLen+1)
	}

	// Set initial values
	for i := int64(0); i <= targetLen; i++ {
		dp[i][0] = i
	}
	for j := int64(0); j <= compareLen; j++ {
		dp[0][j] = j
	}

	// Fill the DP table
	for i := int64(1); i <= targetLen; i++ {
		for j := int64(1); j <= compareLen; j++ {
			cost := int64(0)
			if target[i-1] != compare[j-1] {
				cost = 1
			}

			dp[i][j] = int64(math.Min(float64(dp[i-1][j]+1), math.Min(float64(dp[i][j-1]+1), float64(dp[i-1][j-1]+cost))))
		}
	}

	// Return the Levenshtein distance
	return dp[targetLen][compareLen]
}

// Similarity calculates the similarity score based on Levenshtein distance
func Similarity(target, compare string) float64 {
	distance := LevenshteinDistance(target, compare)
	maxLen := math.Max(float64(len(target)), float64(len(compare)))

	if maxLen == 0 {
		// Both strings are empty, treat them as identical
		return 1.0
	}

	// Calculate similarity as 1 - (distance / maxLen)
	return 1.0 - float64(distance)/maxLen
}

// Float64Equal checks if two float64 values are equal within a small tolerance.
func Float64Equal(a, b, epsilon float64) bool {
	return math.Abs(a-b) <= epsilon
}
