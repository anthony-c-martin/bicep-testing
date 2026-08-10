package biceprpcclient

import "testing"

func TestVersionAtLeast(t *testing.T) {
	tests := []struct {
		actual   string
		minimum  string
		expected bool
	}{
		{actual: "0.46.1", minimum: "0.36.1", expected: true},
		{actual: "0.36.1", minimum: "0.36.1", expected: true},
		{actual: "0.25.3", minimum: "0.36.1", expected: false},
		{actual: "0.46.1+abc", minimum: "0.46.1", expected: true},
	}
	for _, test := range tests {
		if actual := versionAtLeast(test.actual, test.minimum); actual != test.expected {
			t.Errorf("versionAtLeast(%q, %q) = %t, want %t", test.actual, test.minimum, actual, test.expected)
		}
	}
}
