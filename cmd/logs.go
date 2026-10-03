package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var tail int
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs <service>",
		Short: "Tails logs for a target compose service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service := args[0]
			fmt.Printf("Fetching logs for %s (tail: %d, follow: %v)\n", service, tail, follow)
			return nil
		},
	}

	cmd.Flags().IntVarP(&tail, "tail", "t", 100, "Number of lines to show from the end of the logs")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")

	return cmd
}
