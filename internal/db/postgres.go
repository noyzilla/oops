package db

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// BuildPostgresCreateSQL builds queries to create user and database with privileges
func BuildPostgresCreateSQL(dbName, username, password string) string {
	return fmt.Sprintf(
		"DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '%s') THEN "+
			"CREATE ROLE \"%s\" WITH LOGIN PASSWORD '%s'; END IF; END $$; "+
			"CREATE DATABASE \"%s\" OWNER \"%s\"; "+
			"GRANT ALL PRIVILEGES ON DATABASE \"%s\" TO \"%s\"; "+
			"\\c \"%s\"; GRANT ALL ON SCHEMA public TO \"%s\";",
		username, username, password, dbName, username, dbName, username, dbName, username,
	)
}

// BuildPostgresPasswdSQL builds query to rotate password
func BuildPostgresPasswdSQL(username, newPassword string) string {
	return fmt.Sprintf("ALTER ROLE \"%s\" WITH PASSWORD '%s';", username, newPassword)
}

// ExecutePostgres runs a query inside the target PostgreSQL container
func ExecutePostgres(ctx context.Context, containerTarget, query string) (string, error) {
	if containerTarget == "" {
		containerTarget = "postgres"
	}

	pgUser := InspectContainerEnv(ctx, containerTarget, "POSTGRES_USER")
	if pgUser == "" {
		pgUser = os.Getenv("POSTGRES_USER")
	}
	if pgUser == "" {
		pgUser = "postgres"
	}

	pgPass := InspectContainerEnv(ctx, containerTarget, "POSTGRES_PASSWORD")
	if pgPass == "" {
		pgPass = os.Getenv("POSTGRES_PASSWORD")
	}

	var execArgs []string
	if pgPass != "" {
		execArgs = []string{"exec", "-i", "-e", "PGPASSWORD=" + pgPass, containerTarget, "psql", "-U", pgUser, "-c", query}
	} else {
		execArgs = []string{"exec", "-i", containerTarget, "psql", "-U", pgUser, "-c", query}
	}

	cmd := exec.CommandContext(ctx, "docker", execArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("postgres query execution failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}
