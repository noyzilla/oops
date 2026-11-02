package accesslog

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// NginxDecoder decodes Nginx access logs formatted in JSON or standard Common/Combined text.
type NginxDecoder struct {
	combinedRegex *regexp.Regexp
}

// NewNginxDecoder returns a new NginxDecoder instance.
func NewNginxDecoder() *NginxDecoder {
	// Standard combined format: IP - - [date] "METHOD PATH PROTO" status size "referer" "agent"
	re := regexp.MustCompile(`^(\S+)\s+\S+\s+\S+\s+\[([^\]]+)\]\s+"([A-Z]+)\s+([^"\s]+)(?:\s+HTTP/[0-9.]+)?(?:")\s+(\d{3})\s+(\d+|-)(?:\s+"([^"]*)"\s+"([^"]*)")?(?:\s+([\d.]+))?`)
	return &NginxDecoder{
		combinedRegex: re,
	}
}

type nginxJSONEntry struct {
	TimeLocal    string      `json:"time_local"`
	TimeISO      string      `json:"time_iso8601"`
	RemoteAddr   string      `json:"remote_addr"`
	Host         string      `json:"http_host"`
	ServerName   string      `json:"server_name"`
	Request      string      `json:"request"`
	Method       string      `json:"request_method"`
	URI          string      `json:"request_uri"`
	Status       interface{} `json:"status"`
	RequestTime  interface{} `json:"request_time"`
	Scheme       string      `json:"scheme"`
	HTTPS        string      `json:"https"`
	Level        string      `json:"level"`
	Message      string      `json:"message"`
}

func (d *NginxDecoder) Decode(line []byte) (*AccessRecord, error) {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return nil, fmt.Errorf("empty line")
	}

	record := &AccessRecord{
		Raw: line,
	}

	// 1. Try JSON format first
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var entry nginxJSONEntry
		if err := json.Unmarshal([]byte(trimmed), &entry); err == nil {
			if entry.Request != "" || entry.URI != "" {
				record.IsSystem = false
				record.ClientIP = entry.RemoteAddr
				record.Host = entry.Host
				if record.Host == "" {
					record.Host = entry.ServerName
				}
				record.Method = strings.ToUpper(entry.Method)
				record.Path = entry.URI

				if record.Method == "" && entry.Request != "" {
					parts := strings.Fields(entry.Request)
					if len(parts) > 0 {
						record.Method = parts[0]
					}
					if len(parts) > 1 && record.Path == "" {
						record.Path = parts[1]
					}
				}

				record.Status = parseStatusInt(entry.Status)
				record.Latency = parseRequestTime(entry.RequestTime)
				record.Timestamp = parseNginxTime(entry.TimeISO, entry.TimeLocal)

				record.Scheme = "http"
				if entry.Scheme != "" {
					record.Scheme = entry.Scheme
				} else if entry.HTTPS == "on" {
					record.Scheme = "https"
				}

				record.Level = calculateHTTPLevel(record.Status)
				return record, nil
			}

			// System log in JSON
			record.IsSystem = true
			record.Level = strings.ToUpper(entry.Level)
			if record.Level == "" {
				record.Level = "INFO"
			}
			record.Message = entry.Message
			if record.Message == "" {
				record.Message = trimmed
			}
			record.Timestamp = time.Now()
			return record, nil
		}
	}

	// 2. Try Combined / Common text format
	matches := d.combinedRegex.FindStringSubmatch(trimmed)
	if len(matches) >= 6 {
		record.IsSystem = false
		record.ClientIP = matches[1]
		if t, err := time.Parse("02/Jan/2006:15:04:05 -0700", matches[2]); err == nil {
			record.Timestamp = t
		} else {
			record.Timestamp = time.Now()
		}
		record.Method = matches[3]
		record.Path = matches[4]
		record.Status, _ = strconv.Atoi(matches[5])
		record.Scheme = "http"
		record.Level = calculateHTTPLevel(record.Status)

		if len(matches) >= 10 && matches[9] != "" {
			if sec, err := strconv.ParseFloat(matches[9], 64); err == nil {
				record.Latency = time.Duration(sec * float64(time.Second))
			}
		}
		return record, nil
	}

	// 3. Fallback: Nginx error log or startup notice:
	// e.g. 2026/11/02 12:05:01 [error] 123#123: *1 open() failed ...
	record.IsSystem = true
	record.Level = "INFO"
	record.Timestamp = time.Now()
	record.Message = trimmed

	if strings.Contains(trimmed, "[error]") {
		record.Level = "ERROR"
	} else if strings.Contains(trimmed, "[warn]") {
		record.Level = "WARN"
	} else if strings.Contains(trimmed, "[notice]") || strings.Contains(trimmed, "[info]") {
		record.Level = "INFO"
	}

	return record, nil
}

func parseStatusInt(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		st, _ := strconv.Atoi(val)
		return st
	}
	return 0
}

func parseRequestTime(v interface{}) time.Duration {
	switch val := v.(type) {
	case float64:
		return time.Duration(val * float64(time.Second))
	case string:
		if sec, err := strconv.ParseFloat(val, 64); err == nil {
			return time.Duration(sec * float64(time.Second))
		}
	}
	return 0
}

func parseNginxTime(iso, local string) time.Time {
	if iso != "" {
		if t, err := time.Parse(time.RFC3339, iso); err == nil {
			return t
		}
	}
	if local != "" {
		if t, err := time.Parse("02/Jan/2006:15:04:05 -0700", local); err == nil {
			return t
		}
	}
	return time.Now()
}

func calculateHTTPLevel(status int) string {
	if status >= 500 {
		return "ERROR"
	}
	if status >= 400 {
		return "WARN"
	}
	return "INFO"
}

