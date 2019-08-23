package localmetrics

import (
	"testing"
)

func TestBoolToString(t *testing.T) {
	tests := []struct {
		tcase    bool
		expected string
	}{
		{true, "1"},
		{false, "0"},
	}
	for _, test := range tests {
		r := allowFirstToString(test.tcase)
		if r != test.expected {
			t.Errorf("Expected %s from a boolean value of %t, but got %s\n", test.expected, test.tcase, r)
		}
	}
}
