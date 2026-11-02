package accesslog

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CaddyDecoder decodes Zap JSON logs emitted by Caddy / Caddy Docker Proxy.
type CaddyDecoder struct{}

// NewCaddyDecoder returns a new CaddyDecoder instance.
func NewCaddyDecoder() *CaddyDecoder {
	return &CaddyDecoder{}
}

type caddyLogEntry struct {
	Level    string          `json:"level"`
	TS       json.RawMessage `json:"ts"`
	Logger   string          `json:"logger"`
	Msg      string          `json:"msg"`
	Duration float64         `json:"duration"`
	Status   int             `json:"status"`
	Size     int64           `json:"size"`
	Request  *caddyRequest   `json:"request"`
}

type caddyRequest struct {
	RemoteIP string                 `json:"remote_ip"`
	ClientIP string                 `json:"client_ip"`
	Proto    string                 `json:"proto"`
	Method   string                 `json:"method"`
	Host     string                 `json:"host"`
	URI      string                 `json:"uri"`
	Headers  map[string]interface{} `json:"headers"`
	TLS      interface{}            `json:"tls"`
}

func (d *CaddyDecoder) Decode(line []byte) (*AccessRecord, error) {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return nil, fmt.Errorf("empty line")
	}

	record := &AccessRecord{
		Raw: line,
	}

	var entry caddyLogEntry
	if err := json.Unmarshal([]byte(trimmed), &entry); err != nil {
		// Non-JSON line from container (e.g. startup banner)
		record.IsSystem = true
		record.Level = "INFO"
		record.Message = trimmed
		record.Timestamp = time.Now()
		return record, nil
	}

	record.Timestamp = parseCaddyTimestamp(entry.TS)
	record.Level = strings.ToUpper(entry.Level)
	if record.Level == "" {
		record.Level = "INFO"
	}

	// Check if this is an HTTP access log
	if entry.Request != nil && (strings.HasPrefix(entry.Logger, "http.log.access") || entry.Msg == "handled request") {
		record.IsSystem = false
		record.Status = entry.Status
		record.Method = strings.ToUpper(entry.Request.Method)
		record.Host = entry.Request.Host
		record.Path = entry.Request.URI
		record.Latency = time.Duration(entry.Duration * float64(time.Second))

		client := entry.Request.ClientIP
		if client == "" {
			client = entry.Request.RemoteIP
		}
		// Strip port if present in IP:port
		if idx := strings.LastIndex(client, ":"); idx != -1 && !strings.Contains(client, "]") {
			client = client[:idx]
		}
		record.ClientIP = client

		record.Scheme = "http"
		if entry.Request.TLS != nil {
			record.Scheme = "https"
		} else if entry.Request.Headers != nil {
			if proto, ok := entry.Request.Headers["X-Forwarded-Proto"]; ok {
				if protoStr, isStr := proto.(string); isStr && strings.EqualFold(protoStr, "https") {
					record.Scheme = "https"
				}
			}
		}

		return record, nil
	}

	// Internal/system log from Caddy (e.g. tls.cache, admin, http.log)
	record.IsSystem = true
	record.Message = entry.Msg
	if entry.Logger != "" {
		record.Message = fmt.Sprintf("[%s] %s", entry.Logger, entry.Msg)
	}
	return record, nil
}

func parseCaddyTimestamp(raw json.RawMessage) time.Time {
	if len(raw) == 0 {
		return time.Now()
	}

	// 1. Try float64 unix epoch (e.g. 1793587883.123)
	var epoch float64
	if err := json.Unmarshal(raw, &epoch); err == nil && epoch > 0 {
		sec := int64(epoch)
		nsec := int64((epoch - float64(sec)) * 1e9)
		return time.Unix(sec, nsec)
	}

	// 2. Try RFC3339 string (e.g. "2026-11-02T12:05:01Z")
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		if t, err := time.Parse(time.RFC3339Nano, str); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339, str); err == nil {
			return t
		}
	}

	return time.Now()
}

