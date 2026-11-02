package accesslog

import (
	"testing"
	"time"
)

func TestFilterCriteria_Matching(t *testing.T) {
	flags := FilterFlags{
		Level:   "error|warn",
		Status:  "^5..",
		Method:  "POST",
		Host:    `.*\.oops`,
		Path:    `^/auth`,
		Latency: ">50ms",
	}

	fc, err := ParseFilterCriteria(flags)
	if err != nil {
		t.Fatalf("failed to parse filter criteria: %v", err)
	}

	// 1. Should match
	matchedRec := &AccessRecord{
		Level:    "ERROR",
		Status:   502,
		Method:   "POST",
		Host:     "api.web.oops",
		Path:     "/auth/login",
		Latency:  85 * time.Millisecond,
		ClientIP: "127.0.0.1",
	}
	if !fc.Matches(matchedRec) {
		t.Errorf("expected record to match filter criteria, but got false")
	}

	// 2. Status mismatch (200 vs ^5..)
	unmatchedStatus := *matchedRec
	unmatchedStatus.Status = 200
	if fc.Matches(&unmatchedStatus) {
		t.Errorf("expected status mismatch to return false")
	}

	// 3. Method mismatch (GET vs POST)
	unmatchedMethod := *matchedRec
	unmatchedMethod.Method = "GET"
	if fc.Matches(&unmatchedMethod) {
		t.Errorf("expected method mismatch to return false")
	}

	// 4. Latency mismatch (< 50ms)
	unmatchedLatency := *matchedRec
	unmatchedLatency.Latency = 10 * time.Millisecond
	if fc.Matches(&unmatchedLatency) {
		t.Errorf("expected latency mismatch to return false")
	}

	// 5. System log without --all flag
	sysRec := &AccessRecord{
		IsSystem: true,
		Level:    "ERROR",
		Message:  "something went wrong",
	}
	if fc.Matches(sysRec) {
		t.Errorf("expected system log to be skipped when IncludeSystem is false")
	}
}

func TestFilterCriteria_IncludeSystem(t *testing.T) {
	flags := FilterFlags{
		All:   true,
		Level: "error",
	}

	fc, err := ParseFilterCriteria(flags)
	if err != nil {
		t.Fatalf("failed to parse filter criteria: %v", err)
	}

	sysErr := &AccessRecord{
		IsSystem: true,
		Level:    "ERROR",
		Message:  "cert renewal failed",
	}
	if !fc.Matches(sysErr) {
		t.Errorf("expected system error to match with --all")
	}

	sysInfo := &AccessRecord{
		IsSystem: true,
		Level:    "INFO",
		Message:  "cert renewal succeeded",
	}
	if fc.Matches(sysInfo) {
		t.Errorf("expected system info to be excluded by --level=error")
	}
}

func TestParseLatencyFilter(t *testing.T) {
	tests := []struct {
		expr     string
		testDur  time.Duration
		expected bool
	}{
		{">10ms", 15 * time.Millisecond, true},
		{">10ms", 5 * time.Millisecond, false},
		{">10ms", 10 * time.Millisecond, false},
		{">=10ms", 10 * time.Millisecond, true},
		{"<5ms", 2 * time.Millisecond, true},
		{"<5ms", 5 * time.Millisecond, false},
		{"<=5ms", 5 * time.Millisecond, true},
		{">10ms,<50ms", 25 * time.Millisecond, true},
		{">10ms,<50ms", 55 * time.Millisecond, false},
		{"10ms..50ms", 10 * time.Millisecond, true},
		{"10ms..50ms", 50 * time.Millisecond, true},
		{"10ms..50ms", 51 * time.Millisecond, false},
		{">50", 60 * time.Millisecond, true}, // bare number defaults to ms
		{">50", 40 * time.Millisecond, false},
	}

	for _, tt := range tests {
		lf, err := ParseLatencyFilter(tt.expr)
		if err != nil {
			t.Errorf("ParseLatencyFilter(%q) error = %v", tt.expr, err)
			continue
		}
		got := lf.Matches(tt.testDur)
		if got != tt.expected {
			t.Errorf("ParseLatencyFilter(%q).Matches(%v) = %v, want %v", tt.expr, tt.testDur, got, tt.expected)
		}
	}
}

