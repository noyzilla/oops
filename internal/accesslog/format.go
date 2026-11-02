package accesslog

import (
	"fmt"
	"time"
)

// ANSI color codes
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiCyan    = "\033[36m"
	ansiMagenta = "\033[35m"
	ansiGray    = "\033[90m"
)

// Formatter renders AccessRecords into human-readable table rows.
type Formatter struct {
	IsTTY bool
}

// NewFormatter creates a Formatter with terminal detection.
func NewFormatter(isTTY bool) *Formatter {
	return &Formatter{
		IsTTY: isTTY,
	}
}

// FormatHeader outputs the table header.
func (f *Formatter) FormatHeader() string {
	header := fmt.Sprintf("%-9s %-6s %-6s %-7s %-16s %-24s %-8s %s",
		"TIME", "LEVEL", "STATUS", "METHOD", "HOST", "PATH", "LATENCY", "CLIENT")
	if f.IsTTY {
		return ansiBold + ansiGray + header + ansiReset
	}
	return header
}

// FormatRecord renders a single normalized AccessRecord.
func (f *Formatter) FormatRecord(r *AccessRecord) string {
	timeStr := r.Timestamp.Format("15:04:05")

	if r.IsSystem {
		lvlStr := f.colorLevel(r.Level)
		statusStr := "[SYS] "
		if f.IsTTY {
			statusStr = ansiGray + "[SYS]" + ansiReset + " "
		}
		msg := r.Message
		return fmt.Sprintf("%-9s %-6s %-6s %-7s %-16s %s",
			timeStr, lvlStr, statusStr, "-", "-", msg)
	}

	lvlStr := f.colorLevel(r.Level)
	statusStr := f.colorStatus(r.Status)
	methodStr := f.colorMethod(r.Method)
	latencyStr := f.colorLatency(r.Latency)

	host := r.Host
	if host == "" {
		host = "-"
	}

	path := r.Path
	if path == "" {
		path = "/"
	}

	client := r.ClientIP
	if client == "" {
		client = "-"
	}

	return fmt.Sprintf("%-9s %-6s %-6s %-7s %-16s %-24s %-8s %s",
		timeStr, lvlStr, statusStr, methodStr, host, path, latencyStr, client)
}

func (f *Formatter) colorLevel(level string) string {
	formatted := fmt.Sprintf("%-5s", level)
	if !f.IsTTY {
		return formatted
	}
	switch level {
	case "ERROR":
		return ansiBold + ansiRed + formatted + ansiReset
	case "WARN":
		return ansiYellow + formatted + ansiReset
	case "DEBUG":
		return ansiMagenta + formatted + ansiReset
	case "INFO":
		return ansiCyan + formatted + ansiReset
	default:
		return formatted
	}
}

func (f *Formatter) colorStatus(status int) string {
	s := strconvItoa(status)
	if !f.IsTTY {
		return fmt.Sprintf("%-3s", s)
	}
	if status >= 500 {
		return ansiBold + ansiRed + fmt.Sprintf("%-3s", s) + ansiReset
	}
	if status >= 400 {
		return ansiYellow + fmt.Sprintf("%-3s", s) + ansiReset
	}
	if status >= 300 {
		return ansiCyan + fmt.Sprintf("%-3s", s) + ansiReset
	}
	if status >= 200 {
		return ansiGreen + fmt.Sprintf("%-3s", s) + ansiReset
	}
	return fmt.Sprintf("%-3s", s)
}

func (f *Formatter) colorMethod(method string) string {
	formatted := fmt.Sprintf("%-6s", method)
	if !f.IsTTY {
		return formatted
	}
	switch method {
	case "GET":
		return ansiCyan + formatted + ansiReset
	case "POST":
		return ansiGreen + formatted + ansiReset
	case "PUT", "PATCH":
		return ansiYellow + formatted + ansiReset
	case "DELETE":
		return ansiRed + formatted + ansiReset
	default:
		return formatted
	}
}

func (f *Formatter) colorLatency(d time.Duration) string {
	str := FormatDuration(d)
	formatted := fmt.Sprintf("%6s", str)
	if !f.IsTTY {
		return formatted
	}
	if d >= 300*time.Millisecond {
		return ansiBold + ansiRed + formatted + ansiReset
	}
	if d >= 50*time.Millisecond {
		return ansiYellow + formatted + ansiReset
	}
	return ansiGray + formatted + ansiReset
}

// FormatDuration formats a time.Duration into concise readable notation (e.g. 14ms, 1.2s, 450µs).
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "0ms"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	sec := float64(d) / float64(time.Second)
	if sec < 10 {
		return fmt.Sprintf("%.1fs", sec)
	}
	return fmt.Sprintf("%.0fs", sec)
}

func strconvItoa(i int) string {
	if i <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", i)
}
