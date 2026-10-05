package dns

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/noyzilla/oops/internal/docker"
)

// RecordType represents the classification of a DNS record
type RecordType string

const (
	RecordTypeStatic   RecordType = "Static"
	RecordTypeExact    RecordType = "Exact"
	RecordTypeWildcard RecordType = "Wildcard"
)

// DNSRecord describes an individual DNS mapping
type DNSRecord struct {
	Hostname  string     `json:"hostname"`
	IP        string     `json:"ip"`
	Type      RecordType `json:"type"`
	Container string     `json:"container,omitempty"`
	Networks  string     `json:"networks,omitempty"`
	Source    string     `json:"source,omitempty"`
}

// DNSReport holds the macro DNS topology and resolved records
type DNSReport struct {
	DaemonEndpoint string      `json:"daemon_endpoint"`
	UpstreamRelays string      `json:"upstream_relays"`
	LocalResolver  string      `json:"local_resolver,omitempty"`
	StaticRecords  []DNSRecord `json:"static_records"`
	DynamicRecords []DNSRecord `json:"dynamic_records"`
}

// CollectStaticRecords parses all static DNS records from file, oops.yml, and environment variables
func CollectStaticRecords(workDir string) []DNSRecord {
	var records []DNSRecord
	seen := make(map[string]bool)

	filePath := FindDNSFilePath(workDir)
	if fi, err := os.Stat(filePath); err == nil && !fi.IsDir() {
		relPath, relErr := filepath.Rel(workDir, filePath)
		sourceLabel := filepath.Base(filePath)
		if relErr == nil && relPath != "" {
			sourceLabel = relPath
		}

		if fileRecords, _, err := ParseDNSFile(filePath); err == nil {
			for host, ip := range fileRecords {
				key := host + "=" + ip.String()
				if !seen[key] {
					seen[key] = true
					recType := RecordTypeStatic
					if strings.HasPrefix(host, ".") {
						recType = RecordTypeWildcard
					}
					records = append(records, DNSRecord{
						Hostname: host,
						IP:       ip.String(),
						Type:     recType,
						Source:   sourceLabel,
					})
				}
			}
		}
	}

	// Also load shared infra DNS records from oops.yml
	if oopsCfg, err := docker.LoadOopsConfig(workDir); err == nil && oopsCfg != nil {
		for _, rawRecord := range oopsCfg.DNS.Records {
			rawRecord = strings.TrimSpace(rawRecord)
			if rawRecord == "" || strings.HasPrefix(rawRecord, "#") {
				continue
			}
			fields := strings.Fields(rawRecord)
			if len(fields) >= 2 {
				domain := strings.TrimSpace(fields[0])
				rawIP := strings.TrimSpace(fields[1])
				if ip := net.ParseIP(rawIP); ip != nil && domain != "" {
					key := domain + "=" + rawIP
					if !seen[key] {
						seen[key] = true
						recType := RecordTypeStatic
						if strings.HasPrefix(domain, ".") {
							recType = RecordTypeWildcard
						}
						records = append(records, DNSRecord{
							Hostname: domain,
							IP:       rawIP,
							Type:     recType,
							Source:   "oops.yml (infra)",
						})
					}
				}
			}
		}
	}

	// Also parse OOPS_DNS_RECORDS
	if rawEnv := os.Getenv("OOPS_DNS_RECORDS"); rawEnv != "" {
		for _, pair := range strings.Split(rawEnv, ",") {
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
					key := host + "=" + rawIP
					if !seen[key] {
						seen[key] = true
						recType := RecordTypeStatic
						if strings.HasPrefix(host, ".") {
							recType = RecordTypeWildcard
						}
						records = append(records, DNSRecord{
							Hostname: host,
							IP:       rawIP,
							Type:     recType,
							Source:   "OOPS_DNS_RECORDS (env)",
						})
					}
				}
			}
		}
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].IP != records[j].IP {
			return CompareIP(records[i].IP, records[j].IP)
		}
		return records[i].Hostname < records[j].Hostname
	})

	return records
}

// CompareIP returns true if ipStr1 is numerically less than ipStr2
func CompareIP(ipStr1, ipStr2 string) bool {
	ip1 := net.ParseIP(ipStr1)
	ip2 := net.ParseIP(ipStr2)
	if ip1 == nil || ip2 == nil {
		return ipStr1 < ipStr2
	}
	ip1 = ip1.To16()
	ip2 = ip2.To16()
	for i := 0; i < len(ip1) && i < len(ip2); i++ {
		if ip1[i] < ip2[i] {
			return true
		}
		if ip1[i] > ip2[i] {
			return false
		}
	}
	return ipStr1 < ipStr2
}

// CollectContainerRecords inspects all running docker containers and maps hostnames
func CollectContainerRecords(ctx context.Context, cli *client.Client) []DNSRecord {
	if cli == nil {
		return nil
	}

	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil
	}

	var records []DNSRecord

	for _, c := range containers {
		inspect, err := cli.ContainerInspect(ctx, c.ID)
		if err != nil {
			continue
		}

		hostname := ExtractContainerHostname(&inspect)
		if hostname == "" || hostname == "localhost" {
			continue
		}

		ip := ExtractContainerIP(&inspect)
		if ip == nil {
			continue
		}

		recType := RecordTypeExact
		if strings.HasPrefix(hostname, ".") {
			recType = RecordTypeWildcard
		}

		var networkNames []string
		if inspect.NetworkSettings != nil {
			for netName := range inspect.NetworkSettings.Networks {
				networkNames = append(networkNames, netName)
			}
		}
		sort.Strings(networkNames)

		cName := strings.TrimPrefix(inspect.Name, "/")

		records = append(records, DNSRecord{
			Hostname:  hostname,
			IP:        ip.String(),
			Type:      recType,
			Container: cName,
			Networks:  strings.Join(networkNames, ", "),
			Source:    "container (" + cName + ")",
		})
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].IP != records[j].IP {
			return CompareIP(records[i].IP, records[j].IP)
		}
		return records[i].Hostname < records[j].Hostname
	})

	return records
}

// LookupRecord resolves a domain against active containers, custom static records, and shared infra records
func LookupRecord(ctx context.Context, workDir string, domain string, dockerCli *client.Client) (*DNSRecord, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return nil, fmt.Errorf("domain or service name cannot be empty")
	}

	// 1. Check running containers first
	if dockerCli != nil {
		containers := CollectContainerRecords(ctx, dockerCli)
		targetLower := strings.ToLower(domain)

		// 1a. Exact hostname or container name match
		for _, c := range containers {
			if strings.EqualFold(c.Hostname, domain) || strings.EqualFold(c.Container, domain) {
				return &c, nil
			}
		}
		// 1b. Container prefix or suffix match (e.g. "caddy" matches "caddy-proxy" or "oops-caddy")
		for _, c := range containers {
			cLower := strings.ToLower(c.Container)
			if strings.HasPrefix(cLower, targetLower+"-") || strings.HasPrefix(cLower, targetLower+"_") ||
				strings.HasSuffix(cLower, "-"+targetLower) || strings.HasSuffix(cLower, "_"+targetLower) {
				return &c, nil
			}
		}
		// 1c. Substring match (e.g. "caddy" in "my-caddy-proxy")
		for _, c := range containers {
			if strings.Contains(strings.ToLower(c.Container), targetLower) {
				return &c, nil
			}
		}
		// 1d. Wildcard hostname match check
		for _, c := range containers {
			if strings.HasPrefix(c.Hostname, ".") {
				apex := strings.TrimPrefix(c.Hostname, ".")
				if strings.EqualFold(domain, apex) || strings.HasSuffix(strings.ToLower(domain), strings.ToLower(c.Hostname)) {
					return &c, nil
				}
			}
		}
	}

	// 2. Check static and infra records
	staticRecords := CollectStaticRecords(workDir)
	for _, r := range staticRecords {
		if strings.EqualFold(r.Hostname, domain) {
			return &r, nil
		}
		if strings.HasPrefix(r.Hostname, ".") {
			apex := strings.TrimPrefix(r.Hostname, ".")
			if strings.EqualFold(domain, apex) || strings.HasSuffix(strings.ToLower(domain), strings.ToLower(r.Hostname)) {
				return &r, nil
			}
		}
	}

	return nil, fmt.Errorf("no container or DNS record found matching %q", domain)
}

// InspectDNSRecords aggregates both static and dynamic DNS records into a report
func InspectDNSRecords(ctx context.Context, workDir string, dockerCli *client.Client) (*DNSReport, error) {
	upstream := os.Getenv("OOPS_DNS_UPSTREAM")
	if upstream == "" {
		if oopsCfg, err := docker.LoadOopsConfig(workDir); err == nil && oopsCfg != nil {
			if upstreams := oopsCfg.DNS.GetUpstreams(); len(upstreams) > 0 {
				upstream = strings.Join(upstreams, ", ")
			}
		}
	}
	if upstream == "" {
		upstream = "1.1.1.1:53, 8.8.8.8:53"
	}

	report := &DNSReport{
		DaemonEndpoint: ":53 (UDP/TCP)",
		UpstreamRelays: upstream,
		StaticRecords:  CollectStaticRecords(workDir),
	}

	if dockerCli != nil {
		report.DynamicRecords = CollectContainerRecords(ctx, dockerCli)
	}

	// Check if resolver file exists on macOS (/etc/resolver/<tld>)
	tld := os.Getenv("OOPS_DNS_TLD")
	if tld == "" {
		tld = "oops"
	}
	resolverPath := filepath.Join("/etc/resolver", tld)
	if data, err := os.ReadFile(resolverPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "nameserver") {
				report.LocalResolver = fmt.Sprintf("%s -> %s", resolverPath, strings.TrimSpace(strings.TrimPrefix(line, "nameserver")))
				break
			}
		}
	}

	return report, nil
}

// RenderDNSTable formats the DNS report into a clean human-readable table
func RenderDNSTable(report *DNSReport) string {
	var buf strings.Builder

	buf.WriteString("==============================================================================\n")
	buf.WriteString("  OOPS DNS & SERVICE DISCOVERY\n")
	buf.WriteString("==============================================================================\n")
	buf.WriteString(fmt.Sprintf("  Daemon Port     : %s\n", report.DaemonEndpoint))
	buf.WriteString(fmt.Sprintf("  Upstream Relays : %s\n", report.UpstreamRelays))
	if report.LocalResolver != "" {
		buf.WriteString(fmt.Sprintf("  Local Resolver  : %s\n", report.LocalResolver))
	}
	buf.WriteString("\n")

	// Custom & Infra DNS Records
	buf.WriteString("CUSTOM & INFRA DNS RECORDS (data/oops/dns.records & oops.yml)\n")
	if len(report.StaticRecords) == 0 {
		buf.WriteString("  (No custom records registered)\n\n")
	} else {
		w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  HOSTNAME\tIP\tTYPE\tSOURCE")
		fmt.Fprintln(w, "  ----------------------------------------------------------------------------")
		for _, r := range report.StaticRecords {
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", r.Hostname, r.IP, r.Type, r.Source)
		}
		w.Flush()
		buf.WriteString("\n")
	}

	// Dynamic Container Records
	buf.WriteString("SERVICE CONTAINER RECORDS (Docker Engine — Managed Dynamically)\n")
	if len(report.DynamicRecords) == 0 {
		buf.WriteString("  (No active container hostnames discovered)\n")
	} else {
		w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  CONTAINER\tHOSTNAME\tIP\tTYPE\tNETWORKS")
		fmt.Fprintln(w, "  ----------------------------------------------------------------------------")
		for _, r := range report.DynamicRecords {
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", r.Container, r.Hostname, r.IP, r.Type, r.Networks)
		}
		w.Flush()
	}

	buf.WriteString("==============================================================================\n")
	return buf.String()
}
