package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/noyzilla/oops/internal/doctor"
	"github.com/noyzilla/oops/internal/remote"
	"github.com/spf13/cobra"
)

var (
	doctorComposeAll bool
)

func newDoctorCmd() *cobra.Command {
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Comprehensive system and workspace health diagnostic and repair suite",
		Long:  "Inspects Compose subnets, IPAM containment, DNS resolvers, directory hierarchies, and cross-capsule network allocations.",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			report := doctor.RunFullAudit(context.Background(), workDir)
			fmt.Fprint(cmd.OutOrStdout(), report.Render())
			if report.HasErrors() {
				return fmt.Errorf("doctor audit reported errors")
			}
			return nil
		},
	}

	doctorCmd.AddCommand(newDoctorComposeCmd())
	doctorCmd.AddCommand(newDoctorDNSCmd())

	return doctorCmd
}

func newDoctorComposeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compose [@<remote>]",
		Short: "Audit Compose subnet containment, IPAM compliance, and static IP reservations",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)
			checkAll, _ := cmd.Flags().GetBool("all")
			defer func() {
				_ = cmd.Flags().Set("all", "false")
			}()

			var issues []doctor.AuditIssue
			title := "Oops Compose Diagnostic"

			if checkAll || len(args) == 0 {
				issues = doctor.AuditComposeAll(workDir)
			} else {
				remoteTarget := strings.TrimPrefix(args[0], "@")
				capsuleDir := remote.CapsuleDir(workDir, remoteTarget)
				manifest, _ := remote.LoadConfig(workDir, remoteTarget)
				declaredSubnet := ""
				if manifest != nil {
					declaredSubnet = manifest.Network.Subnet
				}
				capsuleCompose := filepath.Join(capsuleDir, "compose.yaml")
				title = fmt.Sprintf("Oops Compose Diagnostic (@%s)", remoteTarget)
				issues = doctor.AuditComposeSubnets(capsuleCompose, declaredSubnet)
			}

			report := &doctor.AuditReport{
				Title:  title,
				Issues: issues,
			}

			fmt.Fprint(cmd.OutOrStdout(), report.Render())
			if report.HasErrors() {
				return fmt.Errorf("compose audit reported errors")
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&doctorComposeAll, "all", "a", false, "Audit all compose files across local workspace and all remote capsules")
	return cmd
}

func newDoctorDNSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "Audit macOS DNS resolvers, Single-TLD alignment, and orphaned entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir := ResolveWorkDir(targetDir)

			issues := doctor.AuditResolverEntries(workDir)
			report := &doctor.AuditReport{
				Title:  "Oops DNS & Resolver Diagnostic",
				Issues: issues,
			}

			fmt.Fprint(cmd.OutOrStdout(), report.Render())
			if report.HasErrors() {
				return fmt.Errorf("dns audit reported errors")
			}
			return nil
		},
	}

	return cmd
}

