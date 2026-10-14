package sync

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// DefaultAllowedPatterns represents the fail-closed fallback whitelist.
var DefaultAllowedPatterns = []string{
	"/.env",
	"/compose.yaml",
	"/oops.yaml",
	"/compose*.local.yaml",
	"/config/**",
}

// AllowRuleSet encapsulates the compiled whitelist rules.
type AllowRuleSet struct {
	patterns []string
}

// LoadAllowRuleSet loads rules from a .oopsallow file.
// If the file does not exist, it falls back to DefaultAllowedPatterns.
func LoadAllowRuleSet(allowFilePath string) (*AllowRuleSet, error) {
	f, err := os.Open(allowFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &AllowRuleSet{patterns: DefaultAllowedPatterns}, nil
		}
		return nil, err
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(patterns) == 0 {
		patterns = DefaultAllowedPatterns
	}

	return &AllowRuleSet{patterns: patterns}, nil
}

// IsAllowed evaluates whether a relative file path is permitted by the rule set.
func (r *AllowRuleSet) IsAllowed(relPath string) bool {
	clean := filepath.ToSlash(filepath.Clean(relPath))
	clean = strings.TrimPrefix(clean, "/")

	// Hard-deny invariants
	if clean == "" || clean == "." {
		return false
	}
	if clean == "remote.yaml" || strings.HasSuffix(clean, "/remote.yaml") {
		return false
	}
	if strings.HasPrefix(clean, ".oopsallow") || strings.HasSuffix(clean, "/.oopsallow") {
		return false
	}
	if strings.HasSuffix(clean, ".example") {
		return false
	}
	if clean == ".git" || strings.HasPrefix(clean, ".git/") {
		return false
	}
	if clean == ".scratch" || strings.HasPrefix(clean, ".scratch/") {
		return false
	}
	if clean == "data" || strings.HasPrefix(clean, "data/") {
		return false
	}
	if clean == "backups" || strings.HasPrefix(clean, "backups/") {
		return false
	}
	if strings.HasSuffix(clean, ".log") || strings.HasSuffix(clean, ".sock") {
		return false
	}

	// Match against whitelist patterns
	for _, rawPat := range r.patterns {
		pat := filepath.ToSlash(strings.TrimSpace(rawPat))
		pat = strings.TrimPrefix(pat, "/")

		// Recursive directory pattern (e.g. config/**)
		if strings.HasSuffix(pat, "/**") {
			prefix := strings.TrimSuffix(pat, "/**")
			if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
				return true
			}
			continue
		}

		// Exact match
		if clean == pat {
			return true
		}

		// Wildcard glob match (e.g. compose*.local.yaml)
		if matched, _ := filepath.Match(pat, clean); matched {
			return true
		}

		// Match basename if pattern has no slash (e.g. *.env)
		if !strings.Contains(pat, "/") {
			if matched, _ := filepath.Match(pat, filepath.Base(clean)); matched {
				return true
			}
		}
	}

	return false
}

// Patterns returns the loaded pattern rules.
func (r *AllowRuleSet) Patterns() []string {
	return append([]string(nil), r.patterns...)
}
