package dns

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// LoadHostsFile reads standard /etc/hosts formatted file and registers records into resolver
func LoadHostsFile(filePath string, resolver *Resolver) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	lineNum := 0

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

		rawIP := fields[0]
		ip := net.ParseIP(rawIP)
		if ip == nil {
			log.Printf("[DNS] Warning: invalid IP address %q on line %d in %s", rawIP, lineNum, filePath)
			continue
		}

		for _, host := range fields[1:] {
			host = strings.TrimSpace(host)
			if host != "" {
				resolver.Register("static", host, ip)
				count++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("error reading hosts file %s: %w", filePath, err)
	}

	return count, nil
}

// LoadCustomHosts discovers and loads custom static hosts from stack config and env
func LoadCustomHosts(workDir string) *Resolver {
	resolver := NewResolver()

	// 1. Check custom file path from env
	if customFile := os.Getenv("OOPS_HOSTS_FILE"); customFile != "" {
		if count, err := LoadHostsFile(customFile, resolver); err == nil {
			log.Printf("[DNS] Loaded %d custom static DNS records from %s", count, customFile)
		}
	}

	// 2. Check standard stack config paths: stacks/utils/config/oops/hosts, stacks/edge/config/oops/hosts, config/oops/hosts
	candidatePaths := []string{
		filepath.Join(workDir, "stacks", "utils", "config", "oops", "hosts"),
		filepath.Join(workDir, "stacks", "edge", "config", "oops", "hosts"),
		filepath.Join(workDir, "config", "oops", "hosts"),
	}

	for _, hostsPath := range candidatePaths {
		if fi, err := os.Stat(hostsPath); err == nil && !fi.IsDir() {
			if count, err := LoadHostsFile(hostsPath, resolver); err == nil {
				log.Printf("[DNS] Loaded %d custom static DNS records from %s", count, hostsPath)
				break
			}
		}
	}

	// 3. Check OOPS_DNS_RECORDS environment variable (e.g. "hostmac=192.168.5.2, colima=192.168.64.1")
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
