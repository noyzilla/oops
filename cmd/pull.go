package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull [targets...]",
		Short: "Pulls the latest images for stacks or targeted services",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Pulling images for targets: %v\n", args)
			return nil
		},
	}
}
