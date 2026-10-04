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

// CollectStaticRecords parses all static DNS records from file and environment variables
func CollectStaticRecords(workDir string) []DNSRecord {
	var records []DNSRecord
	seen := make(map[string]bool)

	filePath := FindDNSFilePath(workDir)
	if fi, err := os.Stat(filePath); err == nil && !fi.IsDir() {
		sourceLabel := filepath.Base(filePath)
		if strings.Contains(filePath, "config/oops") {
			sourceLabel = "config/oops/" + filepath.Base(filePath)
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
		return records[i].Hostname < records[j].Hostname
	})

	return records
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
		})
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Hostname < records[j].Hostname
	})

	return records
}

// InspectDNSRecords aggregates both static and dynamic DNS records into a report
func InspectDNSRecords(ctx context.Context, workDir string, dockerCli *client.Client) (*DNSReport, error) {
	upstream := os.Getenv("OOPS_DNS_UPSTREAM")
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

	// Check if resolver file exists on macOS (/etc/resolver/oops)
	if data, err := os.ReadFile("/etc/resolver/oops"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "nameserver") {
				report.LocalResolver = fmt.Sprintf("/etc/resolver/oops -> %s", strings.TrimSpace(strings.TrimPrefix(line, "nameserver")))
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

	// 1. Static DNS Records
	buf.WriteString("STATIC DNS RECORDS (config/oops/dns)\n")
	if len(report.StaticRecords) == 0 {
		buf.WriteString("  (No static records registered)\n\n")
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

	// 2. Dynamic Container Records
	buf.WriteString("DYNAMIC CONTAINER RECORDS (Docker Engine)\n")
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
