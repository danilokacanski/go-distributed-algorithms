package math_test

import (
	"os"
	"testing"

	"github.com/danilokacanski/testing/math"
)

var something string

func TestMain(m *testing.M) {
	// Setup code before running tests
	// e.g., initialize resources, set environment variables, etc.
	something = "test"
	exitCode := m.Run()

	os.Exit(exitCode)

}

func TestMinInt(t *testing.T) {
	t.Cleanup(func() {
		// Any cleanup required
	})
	testCases := []struct {
		name     string
		a        int
		b        int
		expected int
	}{
		{name: "MinInt of 5 and 10", a: 5, b: 10, expected: 5},
		{name: "MinInt of -3 and 2", a: -3, b: 2, expected: -3},
		{name: "MinInt of 0 and 0", a: 0, b: 0, expected: 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := math.MinInt(tc.a, tc.b)
			t.Log(something) // Accessing the variable set in TestMain
			if result != tc.expected {
				t.Errorf("Expected MinInt(%d, %d) to be %d, got %d", tc.a, tc.b, tc.expected, result)
			}
		})
	}
}

func TestMaxInt(t *testing.T) {
	result := math.MaxInt(10, 5)
	if result != 10 {
		t.Errorf("Expected MaxInt(10, 5) to be 10, got %d", result)
	}
}
