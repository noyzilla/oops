package doctor

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/noyzilla/oops/internal/box"
	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/remote"
	"gopkg.in/yaml.v3"
)

type Severity string

const (
	SeverityOK   Severity = "OK"
	SeverityWarn Severity = "WARN"
	SeverityErr  Severity = "ERROR"
)

type AuditIssue struct {
	Scope      string   `json:"scope"`      // "compose", "dns", "workspace", "network"
	Target     string   `json:"target"`     // file, remote, or domain
	Severity   Severity `json:"severity"`   // OK, WARN, ERROR
	Message    string   `json:"message"`    // diagnostic description
	Suggestion string   `json:"suggestion"` // remediation command or advice
}

type AuditReport struct {
	Title  string       `json:"title"`
	Issues []AuditIssue `json:"issues"`
}

func (r *AuditReport) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityErr {
			return true
		}
	}
	return false
}

func (r *AuditReport) HasWarnings() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityWarn {
			return true
		}
	}
	return false
}

// Render formats the report into a clean CLI string
func (r *AuditReport) Render() string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("=== %s ===\n\n", r.Title))

	if len(r.Issues) == 0 {
		buf.WriteString("✓ All checks passed cleanly with zero issues detected.\n")
		return buf.String()
	}

	for _, issue := range r.Issues {
		icon := "✓"
		switch issue.Severity {
		case SeverityWarn:
			icon = "⚠️ "
		case SeverityErr:
			icon = "✗"
		}

		targetStr := ""
		if issue.Target != "" {
			targetStr = fmt.Sprintf("[%s] ", issue.Target)
		}

		buf.WriteString(fmt.Sprintf("%s %s%s\n", icon, targetStr, issue.Message))
		if issue.Suggestion != "" {
			buf.WriteString(fmt.Sprintf("   -> Suggestion: %s\n", issue.Suggestion))
		}
	}

	return buf.String()
}

// ParsedCompose holds the subset of Compose syntax required for doctor audits
type ParsedCompose struct {
	Services map[string]struct {
		Networks yaml.Node `yaml:"networks"`
	} `yaml:"services"`
	Networks map[string]struct {
		Ipam struct {
			Config []struct {
				Subnet  string `yaml:"subnet"`
				Gateway string `yaml:"gateway"`
			} `yaml:"config"`
		} `yaml:"ipam"`
	} `yaml:"networks"`
	XOops struct {
		Network struct {
			Subnet string `yaml:"subnet"`
		} `yaml:"network"`
	} `yaml:"x-oops"`
}

// AuditComposeSubnets inspects a compose file for IPAM and subnet compliance
func AuditComposeSubnets(composePath, parentSubnet string) []AuditIssue {
	var issues []AuditIssue

	data, err := os.ReadFile(composePath)
	if err != nil {
		issues = append(issues, AuditIssue{
			Scope:      "compose",
			Target:     composePath,
			Severity:   SeverityErr,
			Message:    fmt.Sprintf("Cannot read compose file: %v", err),
			Suggestion: "Ensure file exists and has read permissions",
		})
		return issues
	}

	var comp ParsedCompose
	if err := yaml.Unmarshal(data, &comp); err != nil {
		issues = append(issues, AuditIssue{
			Scope:      "compose",
			Target:     composePath,
			Severity:   SeverityErr,
			Message:    fmt.Sprintf("Invalid compose YAML syntax: %v", err),
			Suggestion: "Fix YAML syntax formatting in compose file",
		})
		return issues
	}

	// If parentSubnet is not supplied, check if declared in x-oops
	effectiveParent := parentSubnet
	if effectiveParent == "" && comp.XOops.Network.Subnet != "" {
		effectiveParent = comp.XOops.Network.Subnet
	}
	if effectiveParent != "" {
		if err := docker.ValidateNetworkSubnet(effectiveParent); err != nil {
			issues = append(issues, AuditIssue{
				Scope:      "compose",
				Target:     composePath,
				Severity:   SeverityErr,
				Message:    fmt.Sprintf("Invalid root network subnet %q: %v", effectiveParent, err),
				Suggestion: "Use a valid private IPv4 CIDR (e.g. 10.240.0.0/16)",
			})
		}
	}

	// 1. Audit declared networks
	var detectedSubnets []*net.IPNet
	for netName, netConfig := range comp.Networks {
		for _, cfg := range netConfig.Ipam.Config {
			if cfg.Subnet == "" {
				continue
			}

			_, ipNet, err := net.ParseCIDR(cfg.Subnet)
			if err != nil {
				issues = append(issues, AuditIssue{
					Scope:      "compose",
					Target:     fmt.Sprintf("%s (network: %s)", composePath, netName),
					Severity:   SeverityErr,
					Message:    fmt.Sprintf("Invalid CIDR subnet %q", cfg.Subnet),
					Suggestion: "Use a standard CIDR block (e.g. 10.240.1.0/24)",
				})
				continue
			}

			// Check containment within parent subnet
			if effectiveParent != "" {
				contained, err := docker.IsSubnetContained(effectiveParent, cfg.Subnet)
				if err != nil || !contained {
					issues = append(issues, AuditIssue{
						Scope:      "compose",
						Target:     fmt.Sprintf("%s (network: %s)", composePath, netName),
						Severity:   SeverityErr,
						Message:    fmt.Sprintf("Subnet %s is not contained within parent CIDR %s", cfg.Subnet, effectiveParent),
						Suggestion: fmt.Sprintf("Assign subnets within %s (e.g. 10.240.1.0/24)", effectiveParent),
					})
				}
			}

			// Check for overlapping internal subnets
			for _, prev := range detectedSubnets {
				if prev.Contains(ipNet.IP) || ipNet.Contains(prev.IP) {
					issues = append(issues, AuditIssue{
						Scope:      "compose",
						Target:     fmt.Sprintf("%s (network: %s)", composePath, netName),
						Severity:   SeverityErr,
						Message:    fmt.Sprintf("Subnet %s overlaps with existing network subnet %s", cfg.Subnet, prev.String()),
						Suggestion: "Ensure each internal network has a distinct, non-overlapping subnet",
					})
				}
			}
			detectedSubnets = append(detectedSubnets, ipNet)
		}
	}

	// 2. Audit service static IP assignments
	for svcName, svc := range comp.Services {
		extractServiceStaticIPs(composePath, svcName, &svc.Networks, detectedSubnets, &issues)
	}

	return issues
}

func extractServiceStaticIPs(composePath, svcName string, node *yaml.Node, subnets []*net.IPNet, issues *[]AuditIssue) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(node.Content); i += 2 {
		netNode := node.Content[i]
		valNode := node.Content[i+1]
		if valNode.Kind != yaml.MappingNode {
			continue
		}

		for j := 0; j < len(valNode.Content); j += 2 {
			k := valNode.Content[j].Value
			v := valNode.Content[j+1].Value
			if k == "ipv4_address" && v != "" {
				ip := net.ParseIP(v)
				if ip == nil {
					*issues = append(*issues, AuditIssue{
						Scope:      "compose",
						Target:     fmt.Sprintf("%s (service: %s, net: %s)", composePath, svcName, netNode.Value),
						Severity:   SeverityErr,
						Message:    fmt.Sprintf("Invalid ipv4_address: %s", v),
						Suggestion: "Provide a valid IPv4 address",
					})
					continue
				}

				ip4 := ip.To4()
				if ip4 == nil {
					continue
				}

				// Check .1 Gateway reservation
				if ip4[3] == 1 {
					*issues = append(*issues, AuditIssue{
						Scope:      "compose",
						Target:     fmt.Sprintf("%s (service: %s)", composePath, svcName),
						Severity:   SeverityWarn,
						Message:    fmt.Sprintf("Static IP %s ends with .1 which is reserved for the Docker network gateway", v),
						Suggestion: "Assign host IP starting from .10 or higher",
					})
				}

				// Check .2 DNS reservation
				if ip4[3] == 2 {
					*issues = append(*issues, AuditIssue{
						Scope:      "compose",
						Target:     fmt.Sprintf("%s (service: %s)", composePath, svcName),
						Severity:   SeverityWarn,
						Message:    fmt.Sprintf("Static IP %s ends with .2 which is reserved for the Oops DNS daemon", v),
						Suggestion: "Assign host IP starting from .10 or higher",
					})
				}
			}
		}
	}
}

// AuditComposeAll audits both local and all capsule compose files, checking for cross-remote subnet collisions
func AuditComposeAll(workDir string) []AuditIssue {
	var issues []AuditIssue

	// 1. Audit local compose
	localCompose := filepath.Join(workDir, "compose.yaml")
	if _, err := os.Stat(localCompose); err == nil {
		issues = append(issues, AuditComposeSubnets(localCompose, "")...)
	}

	// 2. Audit capsules
	remotes, _ := remote.ListRemotes(workDir)
	remoteSubnets := make(map[string]string) // subnet -> remoteName

	for _, r := range remotes {
		capsuleDir := remote.CapsuleDir(workDir, r)
		manifest, err := remote.LoadConfig(workDir, r)
		declaredSubnet := ""
		if err == nil && manifest.Network.Subnet != "" {
			declaredSubnet = manifest.Network.Subnet
		}

		if declaredSubnet != "" {
			if existingRemote, exists := remoteSubnets[declaredSubnet]; exists {
				issues = append(issues, AuditIssue{
					Scope:      "network",
					Target:     fmt.Sprintf("@%s and @%s", r, existingRemote),
					Severity:   SeverityErr,
					Message:    fmt.Sprintf("Subnet collision detected: both remotes claim subnet %s", declaredSubnet),
					Suggestion: fmt.Sprintf("Assign distinct subnets (e.g. @%s: 10.240.0.0/16, @%s: 10.241.0.0/16)", existingRemote, r),
				})
			} else {
				remoteSubnets[declaredSubnet] = r
			}
		}

		capsuleCompose := filepath.Join(capsuleDir, "compose.yaml")
		if _, err := os.Stat(capsuleCompose); err == nil {
			issues = append(issues, AuditComposeSubnets(capsuleCompose, declaredSubnet)...)
		}
	}

	return issues
}

// AuditResolverEntries audits macOS /etc/resolver configuration
func AuditResolverEntries(workDir string) []AuditIssue {
	var issues []AuditIssue

	resolvers, err := box.ListMacOSResolvers()
	if err != nil {
		issues = append(issues, AuditIssue{
			Scope:      "dns",
			Target:     "/etc/resolver",
			Severity:   SeverityWarn,
			Message:    fmt.Sprintf("Could not read /etc/resolver: %v", err),
			Suggestion: "Check permissions of /etc/resolver",
		})
		return issues
	}

	keepSet := map[string]bool{
		"oops": true,
	}
	if envTLD := os.Getenv("OOPS_DNS_TLD"); envTLD != "" {
		keepSet[strings.ToLower(strings.TrimSpace(envTLD))] = true
	}
	remotes, _ := remote.ListRemotes(workDir)
	for _, r := range remotes {
		keepSet[strings.ToLower(strings.TrimSpace(r))] = true
	}

	resolverSet := make(map[string]bool)
	for _, r := range resolvers {
		name := strings.ToLower(r)
		resolverSet[name] = true
		if !keepSet[name] {
			issues = append(issues, AuditIssue{
				Scope:      "dns",
				Target:     fmt.Sprintf("/etc/resolver/%s", r),
				Severity:   SeverityWarn,
				Message:    fmt.Sprintf("Orphaned macOS DNS resolver for domain .%s (no active remote or workspace)", r),
				Suggestion: "Run 'oops dns prune' to clean up orphaned resolvers",
			})
		}
	}

	// Check if any active remote is missing a resolver
	for _, r := range remotes {
		name := strings.ToLower(r)
		if !resolverSet[name] {
			issues = append(issues, AuditIssue{
				Scope:      "dns",
				Target:     fmt.Sprintf("/etc/resolver/%s", r),
				Severity:   SeverityWarn,
				Message:    fmt.Sprintf("Missing DNS resolver for registered remote '@%s'", r),
				Suggestion: fmt.Sprintf("Run 'oops remote connect @%s' to configure resolver", r),
			})
		}
	}

	return issues
}

// AuditWorkspace verifies basic workspace health
func AuditWorkspace(workDir string) []AuditIssue {
	var issues []AuditIssue

	// Check services/ or compose.yaml
	hasCompose := false
	if _, err := os.Stat(filepath.Join(workDir, "compose.yaml")); err == nil {
		hasCompose = true
	}
	if _, err := os.Stat(filepath.Join(workDir, "services")); err == nil {
		hasCompose = true
	}

	if !hasCompose {
		issues = append(issues, AuditIssue{
			Scope:      "workspace",
			Target:     workDir,
			Severity:   SeverityErr,
			Message:    "Workspace does not contain 'compose.yaml' or 'services/' directory",
			Suggestion: "Run 'oops box init' or navigate to a valid Oopsbox workspace",
		})
	}

	return issues
}

// RunFullAudit runs comprehensive diagnostics across workspace, compose, and DNS
func RunFullAudit(ctx context.Context, workDir string) *AuditReport {
	report := &AuditReport{
		Title: "Oops Health Diagnostic Report",
	}

	report.Issues = append(report.Issues, AuditWorkspace(workDir)...)
	report.Issues = append(report.Issues, AuditComposeAll(workDir)...)
	report.Issues = append(report.Issues, AuditResolverEntries(workDir)...)

	return report
}
