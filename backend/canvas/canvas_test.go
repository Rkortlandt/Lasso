package canvas

import (
	"testing"
)

func TestCleanCanvasHTML(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Canvas screenreader-only span with (Links to an external site.)",
			input:    `<p><a href="https://example.com" target="_blank"><span>Example Link</span><span class="screenreader-only">&nbsp;(Links to an external site.)</span></a></p>`,
			expected: `<p><a href="https://example.com" target="_blank"><span>Example Link</span></a></p>`,
		},
		{
			name:     "Canvas ui-icon-extlink span",
			input:    `<p><a href="https://google.com">Google<span class="ui-icon ui-icon-extlink ui-icon-inline" title="Links to an external site."></span><span class="screenreader-only">(Links to an external site.)</span></a></p>`,
			expected: `<p><a href="https://google.com">Google</a></p>`,
		},
		{
			name:     "Direct text Links to external site.",
			input:    `<div>Please check this link Links to external site. before class.</div>`,
			expected: `<div>Please check this link before class.</div>`,
		},
		{
			name:     "Direct text (Links to external site)",
			input:    `<a href="https://example.org">Resource (Links to external site)</a>`,
			expected: `<a href="https://example.org">Resource</a>`,
		},
		{
			name:     "Direct text [Links to an external site.]",
			input:    `<p>Read [Links to an external site.] here.</p>`,
			expected: `<p>Read here.</p>`,
		},
		{
			name:     "Empty input",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := cleanCanvasHTML(tc.input)
			if actual != tc.expected {
				t.Errorf("cleanCanvasHTML(%q) = %q, expected %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestNormalizeHexColor(t *testing.T) {
	testCases := []struct {
		input       string
		expected    string
		expectError bool
	}{
		{"#3b82f6", "#3b82f6", false},
		{"3b82f6", "#3b82f6", false},
		{"#FFF", "#fff", false},
		{"fff", "#fff", false},
		{"#10B981", "#10b981", false},
		{"", "", true},
		{"not-a-color", "", true},
		{"#12345", "", true},
		{"#1234567", "", true},
	}

	for _, tc := range testCases {
		res, err := normalizeHexColor(tc.input)
		if tc.expectError {
			if err == nil {
				t.Errorf("normalizeHexColor(%q) expected error, got nil", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("normalizeHexColor(%q) unexpected error: %v", tc.input, err)
			}
			if res != tc.expected {
				t.Errorf("normalizeHexColor(%q) = %q, expected %q", tc.input, res, tc.expected)
			}
		}
	}
}

func TestGradesEqual(t *testing.T) {
	cases := []struct {
		existing string
		incoming string
		expected bool
	}{
		// Exact matches
		{"4/5", "4/5", true},
		{"100%", "100%", true},
		{"", "", true},

		// Fraction exists, incoming is bare score: should preserve fraction (no change)
		{"4/5", "4", true},
		{"1.9/2", "1.9", true},
		{"10/10", "10", true},

		// Incoming has fraction, existing is bare score: should upgrade
		{"4", "4/5", false},

		// Incoming is empty: should preserve existing grade
		{"4/5", "", true},
		{"100%", "", true},

		// Genuine grade change
		{"4/5", "5/5", false},
		{"3/5", "4", false},
	}

	for _, tc := range cases {
		actual := gradesEqual(tc.existing, tc.incoming)
		if actual != tc.expected {
			t.Errorf("gradesEqual(%q, %q) = %v, expected %v", tc.existing, tc.incoming, actual, tc.expected)
		}
	}
}

