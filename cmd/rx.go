package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// HandleDynamicRxCommands handles invocations like:
//   oops rx status
//   oops rx -r lima-debian logs app
//   oops rx -r prod up /db
func HandleDynamicRxCommands(args []string) (bool, error) {
	if len(args) < 2 || args[0] != "rx" {
		return false, nil
	}

	workDir := ResolveGitWorkDir(targetDir)
	serverName, commandArgs := parseRxInvocation(args, workDir)

	if len(commandArgs) == 0 || commandArgs[0] == "--help" || commandArgs[0] == "-h" || commandArgs[0] == "help" {
		printRxHelp()
		return true, nil
	}

	action := commandArgs[0]
	if action == "remote" || action == "deploy" || action == "rx" {
		return true, fmt.Errorf("cannot execute '%s' command inside 'oops rx'", action)
	}

	sshTarget := resolveSSHTarget(workDir, serverName)

	remoteCmd := fmt.Sprintf("cd ~/oopsbox && (oops %s || /var/lib/google/bin/oops %s)",
		strings.Join(commandArgs, " "), strings.Join(commandArgs, " "))

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
		if arg == "-r" || arg == "--remote" {
			if i+1 < len(args) {
				serverName = args[i+1]
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--remote=") {
			serverName = strings.TrimPrefix(arg, "--remote=")
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
	fmt.Println("  oops rx [-r <remote>] <command> [args...]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -r, --remote string   Target remote server name (default: oopsbox or active workspace remote)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  oops rx status                     Check container status on default remote server")
	fmt.Println("  oops rx logs app -f                Follow app container logs on default remote server")
	fmt.Println("  oops rx -r lima-debian status      Check container status on 'lima-debian'")
	fmt.Println("  oops rx -r prod up /db             Start /db stack on 'prod'")
}

func newRxCmd() *cobra.Command {
	var remoteName string

	cmd := &cobra.Command{
		Use:   "rx [-r <remote>] <command> [args...]",
		Short: "Executes commands directly on a remote server over SSH",
		Long:  "Executes any oops operational command (status, logs, up, down, db, backup, etc.) on a remote server over SSH using -r <remote> or default server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			printRxHelp()
			return nil
		},
	}

	cmd.Flags().StringVarP(&remoteName, "remote", "r", "", "Target remote server name (default: oopsbox or active workspace remote)")
	return cmd
}
