package orchestrator

import (
	"strconv"
	"strings"
	"time"
)

// ParseDurationWithDefault parses duration strings (30s, 10m, 1h) or numeric seconds (30, 600)
// If the input is empty or invalid, it returns fallback duration.
func ParseDurationWithDefault(raw string, defaultDur time.Duration) time.Duration {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultDur
	}
	if sec, err := strconv.Atoi(trimmed); err == nil && sec >= 0 {
		return time.Duration(sec) * time.Second
	}
	if d, err := time.ParseDuration(trimmed); err == nil && d >= 0 {
		return d
	}
	return defaultDur
}
