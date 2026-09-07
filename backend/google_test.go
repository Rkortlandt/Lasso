package main

import (
	"net/http"
	"sync"
	"testing"
	"time"
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

func TestGoogleSharedHTTPClient(t *testing.T) {
	if googleSharedHTTPClient == nil {
		t.Fatal("expected googleSharedHTTPClient to be initialized")
	}

	tr, ok := googleSharedHTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected googleSharedHTTPClient.Transport to be *http.Transport")
	}

	if !tr.ForceAttemptHTTP2 {
		t.Errorf("expected ForceAttemptHTTP2 to be true")
	}

	if tr.MaxIdleConns < 100 {
		t.Errorf("expected MaxIdleConns >= 100, got %d", tr.MaxIdleConns)
	}

	if tr.MaxIdleConnsPerHost < 20 {
		t.Errorf("expected MaxIdleConnsPerHost >= 20, got %d", tr.MaxIdleConnsPerHost)
	}
}

func TestWorkerPoolAccumulation(t *testing.T) {
	const totalItems = 50
	const maxConcurrency = 8

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	var savedIDs []int
	var updatedCount, createdCount int

	for i := 0; i < totalItems; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(id int) {
			defer func() {
				<-sem
				wg.Done()
			}()

			// Simulate work
			time.Sleep(2 * time.Millisecond)

			mu.Lock()
			savedIDs = append(savedIDs, id)
			if id%2 == 0 {
				createdCount++
			} else {
				updatedCount++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if len(savedIDs) != totalItems {
		t.Errorf("expected %d saved IDs, got %d", totalItems, len(savedIDs))
	}
	if createdCount+updatedCount != totalItems {
		t.Errorf("expected total counts to match items, got %d", createdCount+updatedCount)
	}
}

func TestDatesEqual(t *testing.T) {
	cases := []struct {
		d1       string
		d2       string
		expected bool
	}{
		// PocketBase format vs Canvas RFC3339 format
		{"2026-08-31 03:59:00.000Z", "2026-08-31T03:59:00Z", true},
		{"2026-08-31T03:59:00Z", "2026-08-31 03:59:00.000Z", true},
		{"2026-09-01 15:00:00.000Z", "2026-09-01T15:00:00Z", true},
		{"2026-09-02 03:59:59.000Z", "2026-09-02T03:59:59Z", true},

		// Date-only matching
		{"2026-09-01", "2026-09-01", true},

		// Empty strings
		{"", "", true},
		{"   ", "", true},
		{"2026-08-31 03:59:00.000Z", "", false},
		{"", "2026-08-31T03:59:00Z", false},

		// Genuinely different timestamps
		{"2026-08-31 03:59:00.000Z", "2026-08-31 04:00:00.000Z", false},
		{"2026-08-31T03:59:00Z", "2026-09-01T03:59:00Z", false},
	}

	for _, tc := range cases {
		actual := datesEqual(tc.d1, tc.d2)
		if actual != tc.expected {
			t.Errorf("datesEqual(%q, %q) = %v, expected %v", tc.d1, tc.d2, actual, tc.expected)
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

