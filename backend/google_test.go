package main

import (
	"testing"
)

func TestExtractCourseTag(t *testing.T) {
	cases := []struct {
		courseName string
		nickname   string
		expected   string
	}{
		{"CS 1122 Fall 2026", "", "CS 1122"},
		{"Physics 2100", "", "Physics 2100"},
		{"Calculus with Technology II", "Calc 2", "Calc 2"},
		{"Short Name", "", "Short Name"},
	}

	for _, tc := range cases {
		res := extractCourseTag(tc.courseName, tc.nickname)
		if res != tc.expected {
			t.Errorf("for (%s, %s) expected %s, got %s", tc.courseName, tc.nickname, tc.expected, res)
		}
	}
}

func TestParseDateString(t *testing.T) {
	_, isDateOnly, err := parseDateString("2026-09-01")
	if err != nil || !isDateOnly {
		t.Errorf("expected date-only parse for 2026-09-01")
	}

	_, isDateOnly2, err := parseDateString("2026-09-01T14:30:00Z")
	if err != nil || isDateOnly2 {
		t.Errorf("expected datetime parse for 2026-09-01T14:30:00Z")
	}
}
