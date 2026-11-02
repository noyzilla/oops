package accesslog

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// FilterFlags holds raw string flags passed from CLI.
type FilterFlags struct {
	Level   string
	Status  string
	Method  string
	Host    string
	Path    string
	Scheme  string
	Client  string
	Latency string
	All     bool
}

// ParseFilterCriteria compiles CLI flag strings into an executable FilterCriteria.
func ParseFilterCriteria(flags FilterFlags) (*FilterCriteria, error) {
	fc := &FilterCriteria{
		IncludeSystem: flags.All,
	}

	compileRegex := func(name, pattern string) (*regexp.Regexp, error) {
		if strings.TrimSpace(pattern) == "" {
			return nil, nil
		}
		// Case-insensitive matching by default for usability
		pat := pattern
		if !strings.HasPrefix(pat, "(?i)") {
			pat = "(?i)" + pat
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, fmt.Errorf("invalid regex for %s: %w", name, err)
		}
		return re, nil
	}

	var err error
	if fc.LevelPattern, err = compileRegex("--level", flags.Level); err != nil {
		return nil, err
	}
	if fc.StatusPattern, err = compileRegex("--status", flags.Status); err != nil {
		return nil, err
	}
	if fc.MethodPattern, err = compileRegex("--method", flags.Method); err != nil {
		return nil, err
	}
	if fc.HostPattern, err = compileRegex("--host", flags.Host); err != nil {
		return nil, err
	}
	if fc.PathPattern, err = compileRegex("--path", flags.Path); err != nil {
		return nil, err
	}
	if fc.SchemePattern, err = compileRegex("--scheme", flags.Scheme); err != nil {
		return nil, err
	}
	if fc.ClientPattern, err = compileRegex("--client", flags.Client); err != nil {
		return nil, err
	}

	if strings.TrimSpace(flags.Latency) != "" {
		lf, err := ParseLatencyFilter(flags.Latency)
		if err != nil {
			return nil, err
		}
		fc.Latency = lf
	}

	return fc, nil
}

// ParseLatencyFilter parses duration comparison expressions such as:
// ">10ms", ">=100ms", "<5ms", "<=1s", ">10ms,<100ms", "10ms..100ms", or bare numbers like ">50".
func ParseLatencyFilter(expr string) (*LatencyFilter, error) {
	clean := strings.TrimSpace(expr)
	if clean == "" {
		return nil, nil
	}

	lf := &LatencyFilter{}

	// Handle range notation "10ms..50ms"
	if strings.Contains(clean, "..") {
		parts := strings.Split(clean, "..")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid latency range: %s", expr)
		}
		minDur, err := parseSingleDuration(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid min duration in %q: %w", expr, err)
		}
		maxDur, err := parseSingleDuration(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid max duration in %q: %w", expr, err)
		}
		lf.Min = &minDur
		lf.MinEqual = true
		lf.Max = &maxDur
		lf.MaxEqual = true
		return lf, nil
	}

	// Handle comma-separated expressions like ">10ms,<100ms"
	clauses := strings.Split(clean, ",")
	for _, clause := range clauses {
		c := strings.TrimSpace(clause)
		if c == "" {
			continue
		}

		if strings.HasPrefix(c, ">=") {
			dur, err := parseSingleDuration(c[2:])
			if err != nil {
				return nil, err
			}
			lf.Min = &dur
			lf.MinEqual = true
		} else if strings.HasPrefix(c, ">") {
			dur, err := parseSingleDuration(c[1:])
			if err != nil {
				return nil, err
			}
			lf.Min = &dur
			lf.MinEqual = false
		} else if strings.HasPrefix(c, "<=") {
			dur, err := parseSingleDuration(c[2:])
			if err != nil {
				return nil, err
			}
			lf.Max = &dur
			lf.MaxEqual = true
		} else if strings.HasPrefix(c, "<") {
			dur, err := parseSingleDuration(c[1:])
			if err != nil {
				return nil, err
			}
			lf.Max = &dur
			lf.MaxEqual = false
		} else {
			// No operator prefix: treat as >= dur
			dur, err := parseSingleDuration(c)
			if err != nil {
				return nil, err
			}
			lf.Min = &dur
			lf.MinEqual = true
		}
	}

	return lf, nil
}

func parseSingleDuration(s string) (time.Duration, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// If bare integer/float without unit, append 'ms' default
	hasUnit := false
	for _, unit := range []string{"ns", "us", "µs", "ms", "s", "m", "h"} {
		if strings.HasSuffix(trimmed, unit) {
			hasUnit = true
			break
		}
	}
	if !hasUnit {
		trimmed += "ms"
	}

	return time.ParseDuration(trimmed)
}

// Matches checks if the duration satisfies the latency bounds.
func (lf *LatencyFilter) Matches(d time.Duration) bool {
	if lf == nil {
		return true
	}
	if lf.Min != nil {
		if lf.MinEqual && d < *lf.Min {
			return false
		}
		if !lf.MinEqual && d <= *lf.Min {
			return false
		}
	}
	if lf.Max != nil {
		if lf.MaxEqual && d > *lf.Max {
			return false
		}
		if !lf.MaxEqual && d >= *lf.Max {
			return false
		}
	}
	return true
}

// Matches evaluates whether an AccessRecord satisfies all configured filter rules.
func (fc *FilterCriteria) Matches(r *AccessRecord) bool {
	if fc == nil || r == nil {
		return true
	}

	// 1. System Log filter
	if r.IsSystem {
		if !fc.IncludeSystem {
			return false
		}
		if fc.LevelPattern != nil && !fc.LevelPattern.MatchString(r.Level) {
			return false
		}
		return true
	}

	// 2. HTTP Access Log filters
	if fc.LevelPattern != nil && !fc.LevelPattern.MatchString(r.Level) {
		return false
	}
	if fc.StatusPattern != nil && !fc.StatusPattern.MatchString(strconv.Itoa(r.Status)) {
		return false
	}
	if fc.MethodPattern != nil && !fc.MethodPattern.MatchString(r.Method) {
		return false
	}
	if fc.HostPattern != nil && !fc.HostPattern.MatchString(r.Host) {
		return false
	}
	if fc.PathPattern != nil && !fc.PathPattern.MatchString(r.Path) {
		return false
	}
	if fc.SchemePattern != nil && !fc.SchemePattern.MatchString(r.Scheme) {
		return false
	}
	if fc.ClientPattern != nil && !fc.ClientPattern.MatchString(r.ClientIP) {
		return false
	}
	if fc.Latency != nil && !fc.Latency.Matches(r.Latency) {
		return false
	}

	return true
}

