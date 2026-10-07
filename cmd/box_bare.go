package cmd

import (
	"fmt"

	"github.com/noyzilla/oops/internal/remote"
	"github.com/spf13/cobra"
)

func newBoxInitBareCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init-bare <bare-path> <oopsbox-path>",
		Short: "Initializes a Git Bare repository with post-receive deployment hooks",
		Long:  "Initializes a server-side Git Bare repository at <bare-path> with post-receive deployment hook pointing to <oopsbox-path>.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			barePath := args[0]
			boxPath := args[1]

			if err := remote.InitBareRepo(barePath, boxPath); err != nil {
				return err
			}

			fmt.Printf("==> Successfully initialized Git Bare Repository at: %s\n", remote.CanonicalPath(barePath))
			fmt.Printf("  Target Workspace : %s\n", remote.CanonicalPath(boxPath))
			fmt.Printf("  Post-receive Hook: %s\n", remote.CanonicalPath(barePath)+"/hooks/post-receive")
			return nil
		},
	}
}
