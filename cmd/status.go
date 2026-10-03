package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Displays formatted status of containers, health, and ports",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Inspecting stack container status...")
			return nil
		},
	}
}
