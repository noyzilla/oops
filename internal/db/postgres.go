package db

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// BuildPostgresCreateSQL builds queries to create user and database with privileges
func BuildPostgresCreateSQL(dbName, password string, topUser string) string {
	sql := fmt.Sprintf(
		"DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '%s') THEN "+
			"CREATE ROLE \"%s\" WITH LOGIN PASSWORD '%s'; END IF; END $$; "+
			"CREATE DATABASE \"%s\" OWNER \"%s\"; "+
			"GRANT ALL PRIVILEGES ON DATABASE \"%s\" TO \"%s\"; "+
			"\\c \"%s\"; GRANT ALL ON SCHEMA public TO \"%s\";",
		dbName, dbName, password, dbName, dbName, dbName, dbName, dbName, dbName,
	)
	if topUser != "" {
		sql += fmt.Sprintf(" GRANT ALL PRIVILEGES ON DATABASE \"%s\" TO \"%s\";", dbName, topUser)
	}
	return sql
}

// BuildPostgresReadonlySQL builds queries to create a readonly user
func BuildPostgresReadonlySQL(dbName, username, password string) string {
	return fmt.Sprintf(
		"DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '%s') THEN "+
			"CREATE ROLE \"%s\" WITH LOGIN PASSWORD '%s'; END IF; END $$; "+
			"GRANT CONNECT ON DATABASE \"%s\" TO \"%s\"; "+
			"\\c \"%s\"; "+
			"GRANT USAGE ON SCHEMA public TO \"%s\"; "+
			"GRANT SELECT ON ALL TABLES IN SCHEMA public TO \"%s\"; "+
			"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO \"%s\";",
		username, username, password, dbName, username, dbName, username, username, username,
	)
}

// BuildPostgresPasswdSQL builds query to rotate password
func BuildPostgresPasswdSQL(username, newPassword string) string {
	return fmt.Sprintf("ALTER ROLE \"%s\" WITH PASSWORD '%s';", username, newPassword)
}

// BuildPostgresRepairSQL builds query to repair role with privileges matching password
func BuildPostgresRepairSQL(dbName, username, password, topUser string, readonly bool) string {
	if readonly {
		return fmt.Sprintf(
			"DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '%s') THEN "+
				"CREATE ROLE \"%s\" WITH LOGIN PASSWORD '%s'; ELSE "+
				"ALTER ROLE \"%s\" WITH PASSWORD '%s'; END IF; END $$; "+
				"GRANT CONNECT ON DATABASE \"%s\" TO \"%s\"; "+
				"\\c \"%s\"; "+
				"GRANT USAGE ON SCHEMA public TO \"%s\"; "+
				"GRANT SELECT ON ALL TABLES IN SCHEMA public TO \"%s\"; "+
				"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO \"%s\";",
			username, username, password, username, password, dbName, username, dbName, username, username, username,
		)
	}
	sql := fmt.Sprintf(
		"DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '%s') THEN "+
			"CREATE ROLE \"%s\" WITH LOGIN PASSWORD '%s'; ELSE "+
			"ALTER ROLE \"%s\" WITH PASSWORD '%s'; END IF; END $$; "+
			"GRANT ALL PRIVILEGES ON DATABASE \"%s\" TO \"%s\"; "+
			"\\c \"%s\"; GRANT ALL ON SCHEMA public TO \"%s\";",
		username, username, password, username, password, dbName, username, dbName, username,
	)
	if topUser != "" {
		sql += fmt.Sprintf(" GRANT ALL PRIVILEGES ON DATABASE \"%s\" TO \"%s\";", dbName, topUser)
	}
	return sql
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

	pgPass := os.Getenv("OOPS_DB_POSTGRES_PASSWORD")
	if pgPass == "" {
		pgPass = InspectContainerEnv(ctx, containerTarget, "POSTGRES_PASSWORD")
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

// CheckPostgresUserOrDBExists checks if a database or role already exists in PostgreSQL
func CheckPostgresUserOrDBExists(ctx context.Context, containerTarget, dbName, username string) (bool, string, error) {
	if username != "" {
		outUser, err := ExecutePostgres(ctx, containerTarget, fmt.Sprintf("SELECT rolname FROM pg_catalog.pg_roles WHERE rolname = '%s';", username))
		if err == nil {
			for _, line := range strings.Split(outUser, "\n") {
				if strings.TrimSpace(line) == username {
					return true, "user", nil
				}
			}
		}
	}
	if dbName != "" {
		outDB, err := ExecutePostgres(ctx, containerTarget, fmt.Sprintf("SELECT datname FROM pg_catalog.pg_database WHERE datname = '%s';", dbName))
		if err == nil {
			for _, line := range strings.Split(outDB, "\n") {
				if strings.TrimSpace(line) == dbName {
					return true, "database", nil
				}
			}
		}
	}
	return false, "", nil
}
