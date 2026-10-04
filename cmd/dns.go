package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/noyzilla/oops/internal/dns"
	"github.com/spf13/cobra"
)

func newDNSCmd() *cobra.Command {
	dnsCmd := &cobra.Command{
		Use:   "dns",
		Short: "Inspect and manage active DNS records, static mappings, and discovery routes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDNSList(cmd)
		},
	}

	dnsCmd.AddCommand(newDNSListCmd())
	dnsCmd.AddCommand(newDNSGetCmd())
	dnsCmd.AddCommand(newDNSSetCmd())
	dnsCmd.AddCommand(newDNSAddCmd())
	dnsCmd.AddCommand(newDNSDelCmd())
	dnsCmd.AddCommand(newDNSReloadCmd())

	return dnsCmd
}

func runDNSList(cmd *cobra.Command) error {
	workDir := ResolveWorkDir(targetDir)

	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		dockerCli = nil
	}

	report, err := dns.InspectDNSRecords(context.Background(), workDir, dockerCli)
	if err != nil {
		return fmt.Errorf("failed to inspect DNS records: %w", err)
	}

	table := dns.RenderDNSTable(report)
	fmt.Fprint(cmd.OutOrStdout(), table)
	return nil
}

func newDNSListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all active DNS records (custom, infra, and dynamic Docker containers)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDNSList(cmd)
		},
	}
}

func newDNSGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <domain>",
		Short: "Query and resolve IP for a specific domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			domain := strings.TrimSpace(args[0])

			dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err != nil {
				dockerCli = nil
			}

			rec, err := dns.LookupRecord(context.Background(), workDir, domain, dockerCli)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), rec.IP)
			return nil
		},
	}
}

func newDNSSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <domain> <ip>",
		Short: "Set or update a custom DNS record in data/oops/dns.records (supports .wildcard)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			domain := strings.TrimSpace(args[0])
			ip := strings.TrimSpace(args[1])

			// Validate against running Docker container hostnames
			if dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation()); err == nil && dockerCli != nil {
				containers := dns.CollectContainerRecords(context.Background(), dockerCli)
				for _, c := range containers {
					if strings.EqualFold(c.Hostname, domain) {
						return fmt.Errorf("cannot add custom DNS record for %q: domain is already registered by active container %q", domain, c.Container)
					}
				}
			}

			targetPath, err := dns.SetStaticDNSRecord(workDir, domain, ip)
			if err != nil {
				return fmt.Errorf("failed to set DNS record: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Set DNS record: %s -> %s (%s)\n", domain, ip, targetPath)
			return nil
		},
	}
}

func newDNSAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "add <domain> <ip>",
		Short:  "Alias for 'dns set'",
		Hidden: true,
		Args:   cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			setCmd := newDNSSetCmd()
			return setCmd.RunE(cmd, args)
		},
	}
}

func newDNSDelCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "del <domain>",
		Aliases: []string{"delete", "rm"},
		Short:   "Delete a custom DNS record from data/oops/dns.records",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			domain := strings.TrimSpace(args[0])

			// Validate that user is not attempting to delete a dynamic container service
			if dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation()); err == nil && dockerCli != nil {
				containers := dns.CollectContainerRecords(context.Background(), dockerCli)
				for _, c := range containers {
					if strings.EqualFold(c.Hostname, domain) {
						return fmt.Errorf("cannot delete container service record %q: it is managed dynamically by Docker container %q", domain, c.Container)
					}
				}
			}

			targetPath, err := dns.DeleteStaticDNSRecord(workDir, domain)
			if err != nil {
				return fmt.Errorf("failed to delete DNS record: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Deleted DNS record: %s (%s)\n", domain, targetPath)
			return nil
		},
	}
}

func newDNSReloadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: "Validate static DNS file and trigger reload across active daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			filePath := dns.FindDNSFilePath(workDir)

			if _, err := os.Stat(filePath); err != nil {
				return fmt.Errorf("DNS configuration file not found at %s", filePath)
			}

			records, warnings, err := dns.ParseDNSFile(filePath)
			for _, w := range warnings {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Warning: %s\n", w)
			}
			if err != nil {
				return fmt.Errorf("failed to parse %s: %w", filePath, err)
			}

			// Touch file modtime to ensure file watcher triggers immediate reload
			now := time.Now()
			_ = os.Chtimes(filePath, now, now)

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Validated %d static DNS records from %s (Hot-reload active via file watcher)\n", len(records), filePath)
			return nil
		},
	}
}
