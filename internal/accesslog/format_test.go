package accesslog

import (
	"strings"
	"testing"
	"time"
)

func TestFormatter_NonTTY(t *testing.T) {
	formatter := NewFormatter(false) // non-TTY, no ANSI

	header := formatter.FormatHeader()
	if strings.Contains(header, "\033[") {
		t.Errorf("FormatHeader() contained ANSI escape codes in non-TTY mode")
	}

	rec := &AccessRecord{
		Timestamp: time.Date(2026, 11, 2, 12, 5, 1, 0, time.UTC),
		Level:     "INFO",
		Status:    200,
		Method:    "GET",
		Host:      "api.web.oops",
		Path:      "/v1/users",
		Latency:   14 * time.Millisecond,
		ClientIP:  "127.0.0.1",
	}

	row := formatter.FormatRecord(rec)
	if strings.Contains(row, "\033[") {
		t.Errorf("FormatRecord() contained ANSI escape codes in non-TTY mode")
	}

	if !strings.Contains(row, "12:05:01") || !strings.Contains(row, "200") || !strings.Contains(row, "api.web.oops") {
		t.Errorf("FormatRecord() missing expected fields: %q", row)
	}
}

func TestFormatter_TTYColors(t *testing.T) {
	formatter := NewFormatter(true) // TTY, has ANSI

	recErr := &AccessRecord{
		Timestamp: time.Date(2026, 11, 2, 12, 5, 5, 0, time.UTC),
		Level:     "ERROR",
		Status:    502,
		Method:    "POST",
		Host:      "api.web.oops",
		Path:      "/auth/login",
		Latency:   450 * time.Millisecond,
		ClientIP:  "127.0.0.1",
	}

	row := formatter.FormatRecord(recErr)
	// Should have red bold ANSI code for 502 / ERROR
	if !strings.Contains(row, ansiRed) {
		t.Errorf("FormatRecord() expected red ANSI code for 502/ERROR: %q", row)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0ms"},
		{500 * time.Microsecond, "500µs"},
		{14 * time.Millisecond, "14ms"},
		{1500 * time.Millisecond, "1.5s"},
		{12 * time.Second, "12s"},
	}

	for _, tt := range tests {
		got := FormatDuration(tt.d)
		if got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

