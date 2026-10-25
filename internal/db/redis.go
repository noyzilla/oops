package db

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// BuildRedisCreateACL builds arguments for creating a Redis ACL user with key pattern access
func BuildRedisCreateACL(username, password, dbName string) []string {
	return []string{
		"ACL", "SETUSER", username,
		"on", ">" + password,
		"~" + dbName + ":*",
		"&*",
		"+@all",
		"-@dangerous",
		"+info",
	}
}

// BuildRedisReadonlyACL builds arguments for creating a readonly Redis ACL user
func BuildRedisReadonlyACL(username, password, dbName string) []string {
	return []string{
		"ACL", "SETUSER", username,
		"on", ">" + password,
		"~" + dbName + ":*",
		"&*",
		"+@read",
		"+info",
	}
}

// BuildRedisPasswdACL builds arguments for updating a Redis ACL user's password, resetting old passwords and ensuring +info
func BuildRedisPasswdACL(username, newPassword string) []string {
	return []string{
		"ACL", "SETUSER", username,
		"on",
		"resetpass",
		">" + newPassword,
		"+info",
	}
}

// BuildRedisDropACL builds arguments for deleting a Redis ACL user
func BuildRedisDropACL(username string) []string {
	return []string{
		"ACL", "DELUSER", username,
	}
}

// BuildRedisRepairACL builds arguments for repairing a Redis user's ACL and password cleanly
func BuildRedisRepairACL(username, password, prefix string, readonly bool) []string {
	if readonly {
		return []string{
			"ACL", "SETUSER", username,
			"on",
			"resetpass",
			">" + password,
			"~" + prefix + ":*",
			"&*",
			"+@read",
			"+info",
		}
	}
	return []string{
		"ACL", "SETUSER", username,
		"on",
		"resetpass",
		">" + password,
		"~" + prefix + ":*",
		"&*",
		"+@all",
		"-@dangerous",
		"+info",
	}
}

// CheckRedisUserExists checks if an ACL user already exists in Redis
func CheckRedisUserExists(ctx context.Context, containerTarget, username string) (bool, error) {
	out, err := ExecuteRedis(ctx, containerTarget, "ACL", "USERS")
	if err != nil {
		return false, err
	}
	for _, u := range strings.Split(out, "\n") {
		if strings.TrimSpace(u) == username {
			return true, nil
		}
	}
	return false, nil
}

// ExecuteRedis runs redis-cli commands inside target Redis container and persists ACL changes
func ExecuteRedis(ctx context.Context, containerTarget string, args ...string) (string, error) {
	if containerTarget == "" {
		containerTarget = "redis"
	}

	rootPass := os.Getenv("OOPS_DB_REDIS_PASSWORD")
	if rootPass == "" {
		rootPass = InspectContainerEnv(ctx, containerTarget, "REDIS_PASSWORD")
	}

	buildCLIArgs := func(cmdArgs []string) []string {
		cliArgs := []string{"exec", "-i", containerTarget, "redis-cli"}
		if rootPass != "" {
			cliArgs = append(cliArgs, "-a", rootPass, "--no-auth-warning")
		}
		cliArgs = append(cliArgs, cmdArgs...)
		return cliArgs
	}

	cmd := exec.CommandContext(ctx, "docker", buildCLIArgs(args)...)
	out, err := cmd.CombinedOutput()
	outputStr := string(out)
	if err != nil {
		return outputStr, fmt.Errorf("redis command failed: %s: %w", strings.TrimSpace(outputStr), err)
	}

	// Persist ACL mutations to users.acl via ACL SAVE
	if len(args) >= 2 && strings.EqualFold(args[0], "ACL") {
		subCmd := strings.ToUpper(args[1])
		if subCmd == "SETUSER" || subCmd == "DELUSER" {
			saveCmd := exec.CommandContext(ctx, "docker", buildCLIArgs([]string{"ACL", "SAVE"})...)
			_ = saveCmd.Run()
		}
	}

	return outputStr, nil
}

