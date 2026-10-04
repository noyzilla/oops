package cmd

import (
	"context"
	"fmt"
	"os"
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
	fmt.Print(table)
	return nil
}

func newDNSListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all active DNS records (static and dynamic Docker containers)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDNSList(cmd)
		},
	}
}

func newDNSAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <domain> <ip>",
		Short: "Add or update a static DNS record in config/oops/dns (supports .wildcard)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			domain := args[0]
			ip := args[1]

			targetPath, err := dns.AddStaticDNSRecord(workDir, domain, ip)
			if err != nil {
				return fmt.Errorf("failed to add DNS record: %w", err)
			}

			fmt.Printf("✓ Added DNS record: %s -> %s (%s)\n", domain, ip, targetPath)
			return nil
		},
	}
}

func newDNSDelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "del <domain>",
		Short: "Delete a static DNS record from config/oops/dns",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			domain := args[0]

			targetPath, err := dns.DeleteStaticDNSRecord(workDir, domain)
			if err != nil {
				return fmt.Errorf("failed to delete DNS record: %w", err)
			}

			fmt.Printf("✓ Deleted DNS record: %s (%s)\n", domain, targetPath)
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
				fmt.Printf("⚠️  Warning: %s\n", w)
			}
			if err != nil {
				return fmt.Errorf("failed to parse %s: %w", filePath, err)
			}

			// Touch file modtime to ensure file watcher triggers immediate reload
			now := time.Now()
			_ = os.Chtimes(filePath, now, now)

			fmt.Printf("✓ Validated %d static DNS records from %s (Hot-reload active via file watcher)\n", len(records), filePath)
			return nil
		},
	}
}
