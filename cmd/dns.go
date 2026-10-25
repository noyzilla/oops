package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/noyzilla/oops/internal/box"
	"github.com/noyzilla/oops/internal/dns"
	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/remote"
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
	dnsCmd.AddCommand(newDNSPruneCmd())
	dnsCmd.AddCommand(newDNSRepairCmd())

	return dnsCmd
}

func runDNSList(cmd *cobra.Command) error {
	workDir := ResolveWorkDir(targetDir)

	dockerCli, err := docker.NewClient()
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

			dockerCli, err := docker.NewClient()
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
			if dockerCli, err := docker.NewClient(); err == nil && dockerCli != nil {
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
		Use:   "del <domain>",
		Short: "Delete a custom DNS record from data/oops/dns.records",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			domain := strings.TrimSpace(args[0])

			// Validate that user is not attempting to delete a dynamic container service
			if dockerCli, err := docker.NewClient(); err == nil && dockerCli != nil {
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

func newDNSPruneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune stale macOS DNS resolver entries that do not belong to active remotes or workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			defer func() {
				_ = cmd.Flags().Set("dry-run", "false")
			}()

			resolvers, err := box.ListMacOSResolvers()
			if err != nil {
				return fmt.Errorf("failed to list resolvers: %w", err)
			}
			if len(resolvers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No DNS resolvers found in /etc/resolver")
				return nil
			}

			// Collect active TLDs to retain
			keepSet := map[string]bool{
				"oops": true,
			}
			if envTLD := os.Getenv("OOPS_DNS_TLD"); envTLD != "" {
				keepSet[strings.ToLower(strings.TrimSpace(envTLD))] = true
			}
			if remotes, err := remote.ListRemotes(workDir); err == nil {
				for _, r := range remotes {
					keepSet[strings.ToLower(strings.TrimSpace(r))] = true
				}
			}

			var stale []string
			for _, r := range resolvers {
				if !keepSet[strings.ToLower(r)] {
					stale = append(stale, r)
				}
			}

			if len(stale) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "✓ All DNS resolvers in /etc/resolver are active and valid")
				return nil
			}

			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Found %d stale DNS resolver(s) that would be pruned: %s\n", len(stale), strings.Join(stale, ", "))
				return nil
			}

			var keepList []string
			for k := range keepSet {
				keepList = append(keepList, k)
			}
			pruned, err := box.PruneMacOSResolvers(keepList)
			if err != nil {
				return fmt.Errorf("failed to prune resolvers: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Pruned %d stale DNS resolver(s): %s\n", len(pruned), strings.Join(pruned, ", "))
			return nil
		},
	}

	cmd.Flags().BoolP("dry-run", "n", false, "Preview resolvers to prune without deleting")
	return cmd
}

func newDNSRepairCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repair",
		Aliases: []string{"fix"},
		Short:   "Repair and synchronize macOS DNS resolver entries (/etc/resolver/*) to active oops container IP",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			defer func() {
				_ = cmd.Flags().Set("dry-run", "false")
			}()

			flagIP, _ := cmd.Flags().GetString("ip")
			targetIP := flagIP
			if targetIP == "" {
				targetIP = box.GetOopsContainerIP()
			}
			if targetIP == "" {
				return fmt.Errorf("cannot determine active oops container IP (is 'edge/oops' running?); specify target IP explicitly with --ip <ip>")
			}

			// Collect active TLDs to synchronize
			activeTLDs := []string{"oops"}
			if envTLD := os.Getenv("OOPS_DNS_TLD"); envTLD != "" {
				activeTLDs = append(activeTLDs, strings.ToLower(strings.TrimSpace(envTLD)))
			}
			if remotes, err := remote.ListRemotes(workDir); err == nil {
				for _, r := range remotes {
					activeTLDs = append(activeTLDs, strings.ToLower(strings.TrimSpace(r)))
				}
			}

			// Deduplicate active TLDs
			seen := make(map[string]bool)
			var uniqueTLDs []string
			for _, t := range activeTLDs {
				if !seen[t] {
					seen[t] = true
					uniqueTLDs = append(uniqueTLDs, t)
				}
			}

			updatedCount := 0
			for _, tld := range uniqueTLDs {
				currentIP, err := box.ReadMacOSResolver(tld)
				if err != nil || currentIP != targetIP {
					if dryRun {
						if err != nil {
							fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Would create /etc/resolver/%s (nameserver %s)\n", tld, targetIP)
						} else {
							fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] Would update /etc/resolver/%s (nameserver %s -> %s)\n", tld, currentIP, targetIP)
						}
					} else {
						if err := box.SetupMacOSResolverWithIP(tld, targetIP); err != nil {
							return fmt.Errorf("failed to configure resolver for .%s: %w", tld, err)
						}
						if err != nil {
							fmt.Fprintf(cmd.OutOrStdout(), "✓ Created /etc/resolver/%s (nameserver %s)\n", tld, targetIP)
						} else {
							fmt.Fprintf(cmd.OutOrStdout(), "✓ Updated /etc/resolver/%s (nameserver %s -> %s)\n", tld, currentIP, targetIP)
						}
					}
					updatedCount++
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "✓ /etc/resolver/%s is up to date (nameserver %s)\n", tld, targetIP)
				}
			}

			if !dryRun {
				_ = box.FlushMacOSDNSCache()
			}

			// Check for orphaned resolvers and advise user if any exist
			resolvers, _ := box.ListMacOSResolvers()
			var orphaned []string
			for _, r := range resolvers {
				if !seen[strings.ToLower(r)] {
					orphaned = append(orphaned, r)
				}
			}
			if len(orphaned) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nTip: Found %d orphaned resolver(s) not in active workspace (%s). Run 'oops dns prune' to remove them.\n", len(orphaned), strings.Join(orphaned, ", "))
			}

			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "\n[dry-run] %d resolver(s) would be repaired/created.\n", updatedCount)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\nRepaired %d resolver(s). macOS DNS cache flushed.\n", updatedCount)
			}

			return nil
		},
	}

	cmd.Flags().BoolP("dry-run", "n", false, "Preview changes without modifying /etc/resolver")
	cmd.Flags().String("ip", "", "Explicit nameserver IP (defaults to running edge/oops IP)")
	return cmd
}
