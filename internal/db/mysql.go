package db

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// BuildMySQLCreateSQL builds query to create database and user with privileges
func BuildMySQLCreateSQL(dbName, username, password string) string {
	return fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; "+
			"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; "+
			"GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%%'; "+
			"FLUSH PRIVILEGES;",
		dbName, username, password, dbName, username,
	)
}

// BuildMySQLPasswdSQL builds query to rotate password
func BuildMySQLPasswdSQL(username, newPassword string) string {
	return fmt.Sprintf(
		"ALTER USER '%s'@'%%' IDENTIFIED BY '%s'; FLUSH PRIVILEGES;",
		username, newPassword,
	)
}

// ExecuteMySQL runs a query inside the target MySQL container
func ExecuteMySQL(ctx context.Context, containerTarget, query string) (string, error) {
	if containerTarget == "" {
		containerTarget = "mysql"
	}

	rootPass := InspectContainerEnv(ctx, containerTarget, "MYSQL_ROOT_PASSWORD")
	if rootPass == "" {
		rootPass = os.Getenv("MYSQL_ROOT_PASSWORD")
	}

	var execArgs []string
	if rootPass != "" {
		execArgs = []string{"exec", "-i", containerTarget, "mysql", "-u", "root", "-p" + rootPass, "-e", query}
	} else {
		execArgs = []string{"exec", "-i", containerTarget, "mysql", "-u", "root", "-e", query}
	}

	cmd := exec.CommandContext(ctx, "docker", execArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("mysql query execution failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}
