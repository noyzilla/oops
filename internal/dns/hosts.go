package dns

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FindDNSFilePath discovers active static DNS records file path
func FindDNSFilePath(workDir string) string {
	if custom := os.Getenv("OOPS_DNS_FILE"); custom != "" {
		return custom
	}
	if custom := os.Getenv("OOPS_HOSTS_FILE"); custom != "" {
		return custom
	}

	candidatePaths := []string{
		filepath.Join(workDir, "config", "oops", "dns"),
		filepath.Join(workDir, "stacks", "utils", "config", "oops", "dns"),
		filepath.Join(workDir, "stacks", "edge", "config", "oops", "dns"),
		filepath.Join(workDir, "config", "oops", "hosts"),
		filepath.Join(workDir, "stacks", "utils", "config", "oops", "hosts"),
		filepath.Join(workDir, "stacks", "edge", "config", "oops", "hosts"),
	}

	for _, p := range candidatePaths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}

	return filepath.Join(workDir, "config", "oops", "dns")
}

// ParseDNSFile reads static DNS records from file in "<domain> <ip>" format (or legacy hosts format)
func ParseDNSFile(filePath string) (map[string]net.IP, []string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	records := make(map[string]net.IP)
	var warnings []string

	scanner := bufio.NewScanner(file)
	lineNum := 0
	isLegacyHosts := strings.HasSuffix(filepath.Base(filePath), "hosts")

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Strip inline comments
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		if isLegacyHosts {
			// Legacy /etc/hosts format: <ip> <domain1> <domain2>...
			rawIP := fields[0]
			ip := net.ParseIP(rawIP)
			if ip == nil {
				warnings = append(warnings, fmt.Sprintf("invalid IP address %q on line %d in %s", rawIP, lineNum, filePath))
				continue
			}
			for _, host := range fields[1:] {
				host = strings.TrimSpace(host)
				if host != "" {
					records[host] = ip
				}
			}
		} else {
			// Oops DNS format: <domain> <ip> (1 row = 1 domain, supports .wildcard)
			domain := strings.TrimSpace(fields[0])
			rawIP := strings.TrimSpace(fields[1])
			ip := net.ParseIP(rawIP)
			if ip == nil {
				warnings = append(warnings, fmt.Sprintf("invalid IP address %q for domain %q on line %d in %s", rawIP, domain, lineNum, filePath))
				continue
			}
			if domain != "" {
				records[domain] = ip
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return records, warnings, fmt.Errorf("error reading %s: %w", filePath, err)
	}

	return records, warnings, nil
}

// LoadHostsFile maintains backward compatibility for loading a file into a resolver
func LoadHostsFile(filePath string, resolver *Resolver) (int, error) {
	records, warnings, err := ParseDNSFile(filePath)
	for _, w := range warnings {
		log.Printf("[DNS] Warning: %s", w)
	}
	if err != nil {
		return 0, err
	}

	for host, ip := range records {
		resolver.Register("static", host, ip)
	}
	return len(records), nil
}

// LoadCustomHosts discovers and loads custom static hosts from stack config and env
func LoadCustomHosts(workDir string) *Resolver {
	resolver := NewResolver()
	filePath := FindDNSFilePath(workDir)

	if fi, err := os.Stat(filePath); err == nil && !fi.IsDir() {
		if count, err := LoadHostsFile(filePath, resolver); err == nil {
			log.Printf("[DNS] Loaded %d custom static DNS records from %s", count, filePath)
		}
	}

	// Environment variable fallback (OOPS_DNS_RECORDS)
	if rawEnv := os.Getenv("OOPS_DNS_RECORDS"); rawEnv != "" {
		pairs := strings.Split(rawEnv, ",")
		envCount := 0
		for _, pair := range pairs {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) != 2 {
				parts = strings.SplitN(pair, ":", 2)
			}
			if len(parts) == 2 {
				host := strings.TrimSpace(parts[0])
				rawIP := strings.TrimSpace(parts[1])
				if ip := net.ParseIP(rawIP); ip != nil && host != "" {
					resolver.Register("static", host, ip)
					envCount++
				}
			}
		}
		if envCount > 0 {
			log.Printf("[DNS] Loaded %d custom DNS records from OOPS_DNS_RECORDS", envCount)
		}
	}

	return resolver
}

// AddStaticDNSRecord adds or updates a static DNS record in config/oops/dns
func AddStaticDNSRecord(workDir, domain, ipStr string) (string, error) {
	domain = strings.TrimSpace(domain)
	ipStr = strings.TrimSpace(ipStr)

	if domain == "" {
		return "", fmt.Errorf("domain cannot be empty")
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %q", ipStr)
	}

	targetPath := FindDNSFilePath(workDir)
	if strings.HasSuffix(filepath.Base(targetPath), "hosts") || !fileExists(targetPath) {
		targetPath = filepath.Join(workDir, "config", "oops", "dns")
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	var lines []string
	updated := false

	if fileExists(targetPath) {
		file, err := os.Open(targetPath)
		if err != nil {
			return "", fmt.Errorf("failed to open %s: %w", targetPath, err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			text := scanner.Text()
			trimmed := strings.TrimSpace(text)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				lines = append(lines, text)
				continue
			}

			fields := strings.Fields(trimmed)
			if len(fields) >= 2 && strings.EqualFold(fields[0], domain) {
				// Update existing entry with aligned spacing
				lines = append(lines, fmt.Sprintf("%-20s %s", domain, ip.String()))
				updated = true
			} else {
				lines = append(lines, text)
			}
		}
	} else {
		lines = append(lines, "# ==============================================================================")
		lines = append(lines, "# Oops Static DNS Records (Domain IP)")
		lines = append(lines, "# ==============================================================================")
	}

	if !updated {
		lines = append(lines, fmt.Sprintf("%-20s %s", domain, ip.String()))
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", targetPath, err)
	}

	return targetPath, nil
}

// DeleteStaticDNSRecord removes a static DNS record from config/oops/dns
func DeleteStaticDNSRecord(workDir, domain string) (string, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", fmt.Errorf("domain cannot be empty")
	}

	targetPath := FindDNSFilePath(workDir)
	if !fileExists(targetPath) {
		return "", fmt.Errorf("DNS configuration file not found at %s", targetPath)
	}

	file, err := os.Open(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to open %s: %w", targetPath, err)
	}
	defer file.Close()

	var lines []string
	found := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			lines = append(lines, text)
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) >= 1 && strings.EqualFold(fields[0], domain) {
			found = true
			continue // skip this line to delete
		}
		lines = append(lines, text)
	}

	if !found {
		return "", fmt.Errorf("record %q not found in %s", domain, targetPath)
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", targetPath, err)
	}

	return targetPath, nil
}

// ReloadStaticRecords parses file and atomic swaps records in resolver
func ReloadStaticRecords(workDir string, resolver *Resolver) (int, []string, error) {
	filePath := FindDNSFilePath(workDir)
	if !fileExists(filePath) {
		resolver.SwapStatic(nil)
		return 0, nil, nil
	}

	records, warnings, err := ParseDNSFile(filePath)
	if err != nil {
		return 0, warnings, err
	}

	resolver.SwapStatic(records)
	return len(records), warnings, nil
}

// WatchDNSConfigFile polls the static records file for changes and auto-reloads resolver
func WatchDNSConfigFile(ctx context.Context, workDir string, resolver *Resolver, interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Second
	}

	var lastModTime time.Time
	var lastPath string

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentPath := FindDNSFilePath(workDir)
			fi, err := os.Stat(currentPath)
			if err != nil {
				continue
			}

			if currentPath != lastPath || fi.ModTime().After(lastModTime) {
				lastPath = currentPath
				lastModTime = fi.ModTime()

				count, warnings, err := ReloadStaticRecords(workDir, resolver)
				for _, w := range warnings {
					log.Printf("[DNS Watcher] Warning: %s", w)
				}
				if err != nil {
					log.Printf("[DNS Watcher] Error reloading %s: %v", currentPath, err)
				} else {
					log.Printf("[DNS Watcher] Successfully auto-reloaded %d static DNS records from %s", count, currentPath)
				}
			}
		}
	}
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
