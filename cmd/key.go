package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/noyzilla/oops/internal/key"
	"github.com/spf13/cobra"
)

func newKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Displays or manages Oops SSH Deploy Key for Git operations",
		Long:  "Displays the public SSH deploy key (~/.oops/id_ed25519.pub). Auto-generates an ed25519 keypair if missing.",
		RunE: func(cmd *cobra.Command, args []string) error {
			keyPath, err := key.DefaultKeyPath()
			if err != nil {
				return err
			}

			pubStr, gen, err := key.EnsureKeyPair(keyPath)
			if err != nil {
				return err
			}

			if gen {
				cmd.Println("==> Generated new ed25519 SSH Deploy Key at:", keyPath)
			}
			cmd.Println("\nPublic Deploy Key:")
			cmd.Println(pubStr)
			cmd.Println("\nAdd this public key as a Read-Only Deploy Key in your Git repository settings.")
			return nil
		},
	}

	cmd.AddCommand(newKeyResetCmd(), newKeySetCmd())
	return cmd
}

func newKeyResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Regenerates a new ed25519 SSH Deploy Key pair",
		RunE: func(cmd *cobra.Command, args []string) error {
			keyPath, err := key.DefaultKeyPath()
			if err != nil {
				return err
			}

			pubStr, err := key.GenerateKeyPair(keyPath)
			if err != nil {
				return err
			}

			cmd.Println("✓ Regenerated new ed25519 SSH Deploy Key at:", keyPath)
			cmd.Println("\nPublic Deploy Key:")
			cmd.Println(pubStr)
			cmd.Println("\nAdd this new public key to your Git repository settings.")
			return nil
		},
	}
}

func newKeySetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set [private_key_content]",
		Short: "Sets a custom SSH private key for Oops Git operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			keyPath, err := key.DefaultKeyPath()
			if err != nil {
				return err
			}

			var privInput string
			if len(args) > 0 {
				privInput = args[0]
			} else {
				cmd.Println("Paste private key PEM content (end input with empty line or Ctrl+D):")
				scanner := bufio.NewScanner(os.Stdin)
				var lines []string
				for scanner.Scan() {
					line := scanner.Text()
					if line == "" && len(lines) > 0 && strings.Contains(lines[len(lines)-1], "END") {
						break
					}
					lines = append(lines, line)
				}
				privInput = strings.Join(lines, "\n")
			}

			pubStr, err := key.SetKeyPair(keyPath, privInput)
			if err != nil {
				return fmt.Errorf("failed setting key: %w", err)
			}

			cmd.Println("✓ Successfully saved SSH Deploy Key to:", keyPath)
			cmd.Println("\nDerived Public Key:")
			cmd.Println(pubStr)
			return nil
		},
	}
}
