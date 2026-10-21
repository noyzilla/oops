package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/noyzilla/oops/internal/remote"
	"github.com/spf13/cobra"
)

// HandleDynamicRxCommands handles invocations like:
//   oops rx ps
//   oops rx @lima-debian logs app
//   oops rx @prod up /db
func HandleDynamicRxCommands(args []string) (bool, error) {
	if len(args) < 2 || args[0] != "rx" {
		return false, nil
	}

	workDir := ResolveWorkDir(targetDir)
	serverName, commandArgs := parseRxInvocation(args, workDir)

	if len(commandArgs) == 0 || commandArgs[0] == "--help" || commandArgs[0] == "-h" || commandArgs[0] == "help" {
		printRxHelp()
		return true, nil
	}

	action := commandArgs[0]
	if action == "remote" || action == "deploy" || action == "rx" {
		return true, fmt.Errorf("cannot execute '%s' command inside 'oops rx'", action)
	}

	boxPath := "~/oopsbox"
	cfg, err := remote.LoadConfig(workDir, serverName)
	if err == nil && cfg.Path != "" {
		boxPath = cfg.Path
	}
	sshTarget := resolveSSHTarget(workDir, serverName)

	remoteCmd := fmt.Sprintf("cd %s && if command -v oops >/dev/null 2>&1; then oops %s; elif [ -f /var/lib/google/bin/oops ]; then /var/lib/google/bin/oops %s; else oops %s; fi",
		boxPath, strings.Join(commandArgs, " "), strings.Join(commandArgs, " "), strings.Join(commandArgs, " "))

	sshArgs := []string{"-o", "StrictHostKeyChecking=accept-new", "-t", sshTarget, remoteCmd}

	cmd := exec.Command("ssh", sshArgs...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return true, cmd.Run()
}

func parseRxInvocation(args []string, workDir string) (serverName string, commandArgs []string) {
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "@") && serverName == "" {
			serverName = strings.TrimPrefix(arg, "@")
			continue
		}
		commandArgs = append(commandArgs, arg)
	}

	if serverName == "" {
		serverName = resolveDefaultServer(workDir)
	}

	return serverName, commandArgs
}

func printRxHelp() {
	fmt.Println("Oops Remote Execution (rx)")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  oops rx [@<remote>] <oops-cmd> [args...]")
	fmt.Println()
	fmt.Println("Description:")
	fmt.Println("  Forwards an oops command directly to the remote server over SSH")
	fmt.Println("  (e.g. 'oops rx ps' executes 'oops ps' inside the remote workspace).")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  oops rx ps                         Runs 'oops ps' on default remote server")
	fmt.Println("  oops rx logs app -f                Follows app container logs on default remote server")
	fmt.Println("  oops rx @prod up /db               Runs 'oops up /db' on '@prod'")
}

func newRxCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rx [@<remote>] <oops-cmd> [args...]",
		Short: "Forwards an oops command directly to a remote server over SSH",
		Long:  "Executes oops operational subcommands (ps, logs, up, down, db, backup, etc.) on a remote server over SSH inside the target workspace directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			printRxHelp()
			return nil
		},
	}
}
