package accesslog

import (
	"strings"
)

// AutoDecoder dynamically attempts decoding across supported reverse proxy engines.
type AutoDecoder struct {
	caddy *CaddyDecoder
	nginx *NginxDecoder
	hint  string
}

// NewAutoDecoder creates a new AutoDecoder with an optional proxy hint (e.g. "caddy", "nginx").
func NewAutoDecoder(hint string) *AutoDecoder {
	return &AutoDecoder{
		caddy: NewCaddyDecoder(),
		nginx: NewNginxDecoder(),
		hint:  strings.ToLower(hint),
	}
}

// Decode attempts to decode the line using the hinted or best-matching decoder.
func (a *AutoDecoder) Decode(line []byte) (*AccessRecord, error) {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return nil, nil
	}

	// If explicitly hinted as nginx, use nginx decoder
	if strings.Contains(a.hint, "nginx") {
		return a.nginx.Decode(line)
	}

	// If hinted as caddy, use caddy decoder
	if strings.Contains(a.hint, "caddy") {
		return a.caddy.Decode(line)
	}

	// Auto-detection:
	// If it starts with JSON '{', try caddy first, then nginx json
	if strings.HasPrefix(trimmed, "{") {
		rec, err := a.caddy.Decode(line)
		if err == nil && !rec.IsSystem {
			return rec, nil
		}
		// Try nginx json if caddy marked it system but nginx might parse it as access log
		if nginxRec, nErr := a.nginx.Decode(line); nErr == nil && !nginxRec.IsSystem {
			return nginxRec, nil
		}
		if err == nil {
			return rec, nil
		}
	}

	// Fallback to nginx decoder (handles standard combined text format)
	return a.nginx.Decode(line)
}

