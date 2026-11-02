package accesslog

import (
	"regexp"
	"time"
)

// AccessRecord represents a normalized HTTP access log event across reverse proxies.
type AccessRecord struct {
	Timestamp time.Time     `json:"timestamp"`
	Level     string        `json:"level"`     // INFO, WARN, ERROR, DEBUG
	Status    int           `json:"status"`    // HTTP status code (200, 404, 502, etc.)
	Method    string        `json:"method"`    // GET, POST, PUT, DELETE, etc.
	Host      string        `json:"host"`      // Hostname / domain (e.g. api.web.oops)
	Path      string        `json:"path"`      // Request URI / path (e.g. /v1/users)
	Scheme    string        `json:"scheme"`    // http, https
	Latency   time.Duration `json:"latency"`   // Duration of request processing
	ClientIP  string        `json:"client_ip"` // Remote client IP
	IsSystem  bool          `json:"is_system"` // True if non-HTTP proxy system/runtime message
	Message   string        `json:"message"`   // Message for system log or raw entry
	Raw       []byte        `json:"raw"`       // Original raw line
}

// LatencyFilter specifies numeric duration comparison rules (>10ms, <5ms, range).
type LatencyFilter struct {
	Min      *time.Duration
	Max      *time.Duration
	MinEqual bool
	MaxEqual bool
}

// FilterCriteria encapsulates compiled regexes and duration bounds.
type FilterCriteria struct {
	LevelPattern  *regexp.Regexp
	StatusPattern *regexp.Regexp
	MethodPattern *regexp.Regexp
	HostPattern   *regexp.Regexp
	PathPattern   *regexp.Regexp
	SchemePattern *regexp.Regexp
	ClientPattern *regexp.Regexp
	Latency       *LatencyFilter
	IncludeSystem bool // true if --all flag is set
}

// Decoder parses a raw log line into a normalized AccessRecord.
type Decoder interface {
	Decode(line []byte) (*AccessRecord, error)
}

