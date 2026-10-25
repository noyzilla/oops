package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/noyzilla/oops/internal/db"
	"github.com/spf13/cobra"
)

func newDBCmd() *cobra.Command {
	var noExport bool

	cmd := &cobra.Command{
		Use:   "db <engine>[:<target>] <action> [args...]",
		Short: "Database provisioning and credentials management for mysql, postgres, or redis",
		Long: `Manages database and user provisioning for supported engines.

Engines:
  mysql     MySQL relational database
  postgres  PostgreSQL relational database
  redis     Redis in-memory store and cache

Actions:
  create    Create a new database and user
  readonly  Create a read-only user for an existing database
  passwd    Update password for a database user
  list      List databases and users
  drop      Drop a database and its associated user`,
		Example: `  oops db mysql create myapp
  oops db postgres readonly myapp_ro
  oops db redis list
  oops db mysql passwd myapp newsecret
  oops db postgres drop myapp`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}

			workDir := ResolveWorkDir(targetDir)
			if err := ValidateActiveBox(workDir); err != nil {
				return err
			}

			engineSpec := args[0]
			action := strings.ToLower(args[1])
			remainingArgs := args[2:]

			parts := strings.SplitN(engineSpec, ":", 2)
			engine := strings.ToLower(parts[0])
			var containerTarget string
			if len(parts) == 2 {
				containerTarget = parts[1]
			}

			switch action {
			case "create":
				if len(remainingArgs) < 1 {
					if engine == "redis" {
						return fmt.Errorf("usage: oops db %s create <prefix> [password]", engineSpec)
					}
					return fmt.Errorf("usage: oops db %s create <db_name> [password]", engineSpec)
				}
				dbName := remainingArgs[0]
				userName := dbName
				
				dbParts := strings.Split(dbName, "_")
				if engine != "redis" && len(dbParts) > 2 {
					return fmt.Errorf("database name '%s' exceeds maximum allowed 2 levels (only one underscore allowed)", dbName)
				}
				isLevel1 := len(dbParts) == 1
				var topUser string
				if !isLevel1 {
					topUser = dbParts[0]
				}

				var password string
				if len(remainingArgs) >= 2 && remainingArgs[1] != "" {
					password = remainingArgs[1]
				} else {
					var err error
					password, err = db.GeneratePassword(20)
					if err != nil {
						return err
					}
				}

				if engine == "mysql" {
					if containerTarget == "" {
						containerTarget = "mysql"
					}
					sql := db.BuildMySQLCreateSQL(dbName, password, isLevel1)
					_, err := db.ExecuteMySQL(context.Background(), containerTarget, sql)
					if err != nil {
						return err
					}
				} else if engine == "postgres" {
					if containerTarget == "" {
						containerTarget = "postgres"
					}
					sql := db.BuildPostgresCreateSQL(dbName, password, topUser)
					_, err := db.ExecutePostgres(context.Background(), containerTarget, sql)
					if err != nil {
						return err
					}
				} else if engine == "redis" {
					if containerTarget == "" {
						containerTarget = "redis"
					}
					aclArgs := db.BuildRedisCreateACL(userName, password, dbName)
					_, err := db.ExecuteRedis(context.Background(), containerTarget, aclArgs...)
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("unsupported database engine: %s", engine)
				}

				if engine == "redis" {
					fmt.Printf("Prefix: %s\nUser: %s\nPassword: %s\n", dbName, userName, password)
				} else {
					fmt.Printf("Database: %s\nUser: %s\nPassword: %s\n", dbName, userName, password)
				}
				
				if !noExport {
					if err := writeEnvFile(workDir, engine, containerTarget, dbName, userName, password); err != nil {
						fmt.Printf("Warning: failed to export env file: %v\n", err)
					}
				}
				
				return nil

			case "readonly":
				if len(remainingArgs) < 2 {
					if engine == "redis" {
						return fmt.Errorf("usage: oops db %s readonly <prefix> <app_name> [password]", engineSpec)
					}
					return fmt.Errorf("usage: oops db %s readonly <db_name> <app_name> [password]", engineSpec)
				}
				dbName := remainingArgs[0]
				appName := remainingArgs[1]
				
				dbParts := strings.Split(dbName, "_")
				if engine != "redis" && len(dbParts) > 2 {
					return fmt.Errorf("database name '%s' exceeds maximum allowed 2 levels (only one underscore allowed)", dbName)
				}
				
				userName := fmt.Sprintf("%s__ro_%s", dbName, appName)

				var password string
				if len(remainingArgs) >= 3 && remainingArgs[2] != "" {
					password = remainingArgs[2]
				} else {
					var err error
					password, err = db.GeneratePassword(20)
					if err != nil {
						return err
					}
				}

				if engine == "mysql" {
					if containerTarget == "" {
						containerTarget = "mysql"
					}
					sql := db.BuildMySQLReadonlySQL(dbName, userName, password)
					_, err := db.ExecuteMySQL(context.Background(), containerTarget, sql)
					if err != nil {
						return err
					}
				} else if engine == "postgres" {
					if containerTarget == "" {
						containerTarget = "postgres"
					}
					sql := db.BuildPostgresReadonlySQL(dbName, userName, password)
					_, err := db.ExecutePostgres(context.Background(), containerTarget, sql)
					if err != nil {
						return err
					}
				} else if engine == "redis" {
					if containerTarget == "" {
						containerTarget = "redis"
					}
					aclArgs := db.BuildRedisReadonlyACL(userName, password, dbName)
					_, err := db.ExecuteRedis(context.Background(), containerTarget, aclArgs...)
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("unsupported database engine: %s", engine)
				}

				if engine == "redis" {
					fmt.Printf("Readonly User: %s\nTarget Prefix: %s\nPassword: %s\n", userName, dbName, password)
				} else {
					fmt.Printf("Readonly User: %s\nTarget DB: %s\nPassword: %s\n", userName, dbName, password)
				}
				
				if !noExport {
					if err := writeEnvFile(workDir, engine, containerTarget, dbName, userName, password); err != nil {
						fmt.Printf("Warning: failed to export env file: %v\n", err)
					}
				}
				
				return nil

			case "passwd", "password":
				if len(remainingArgs) < 1 {
					return fmt.Errorf("usage: oops db %s passwd <user_name> [new_password]", engineSpec)
				}
				userName := remainingArgs[0]
				var newPassword string
				if len(remainingArgs) >= 2 && remainingArgs[1] != "" {
					newPassword = remainingArgs[1]
				} else {
					var err error
					newPassword, err = db.GeneratePassword(20)
					if err != nil {
						return err
					}
				}

				if engine == "mysql" {
					if containerTarget == "" {
						containerTarget = "mysql"
					}
					sql := db.BuildMySQLPasswdSQL(userName, newPassword)
					_, err := db.ExecuteMySQL(context.Background(), containerTarget, sql)
					if err != nil {
						return err
					}
				} else if engine == "postgres" {
					if containerTarget == "" {
						containerTarget = "postgres"
					}
					sql := db.BuildPostgresPasswdSQL(userName, newPassword)
					_, err := db.ExecutePostgres(context.Background(), containerTarget, sql)
					if err != nil {
						return err
					}
				} else if engine == "redis" {
					if containerTarget == "" {
						containerTarget = "redis"
					}
					aclArgs := db.BuildRedisPasswdACL(userName, newPassword)
					_, err := db.ExecuteRedis(context.Background(), containerTarget, aclArgs...)
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("unsupported database engine: %s", engine)
				}

				fmt.Printf("User: %s\nNew Password: %s\n", userName, newPassword)

				if !noExport {
					if err := writePasswdEnvFile(workDir, engine, containerTarget, userName, newPassword); err != nil {
						fmt.Printf("Warning: failed to export env file: %v\n", err)
					}
				}

				return nil

			case "list":
				if engine == "mysql" {
					if containerTarget == "" {
						containerTarget = "mysql"
					}
					out, err := db.ExecuteMySQL(context.Background(), containerTarget, "SHOW DATABASES;")
					if err != nil {
						return err
					}
					fmt.Print(out)
				} else if engine == "postgres" {
					if containerTarget == "" {
						containerTarget = "postgres"
					}
					out, err := db.ExecutePostgres(context.Background(), containerTarget, "\\l")
					if err != nil {
						return err
					}
					fmt.Print(out)
				} else if engine == "redis" {
					if containerTarget == "" {
						containerTarget = "redis"
					}
					out, err := db.ExecuteRedis(context.Background(), containerTarget, "ACL", "LIST")
					if err != nil {
						return err
					}
					fmt.Print(out)
				}
				return nil

			case "drop":
				if len(remainingArgs) < 2 {
					if engine == "redis" {
						return fmt.Errorf("usage: oops db %s drop <prefix> <user_name>", engineSpec)
					}
					return fmt.Errorf("usage: oops db %s drop <db_name> <user_name>", engineSpec)
				}
				dbName := remainingArgs[0]
				userName := remainingArgs[1]

				if engine == "mysql" {
					if containerTarget == "" {
						containerTarget = "mysql"
					}
					sql := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`; DROP USER IF EXISTS '%s'@'%%';", dbName, userName)
					_, err := db.ExecuteMySQL(context.Background(), containerTarget, sql)
					if err != nil {
						return err
					}
				} else if engine == "postgres" {
					if containerTarget == "" {
						containerTarget = "postgres"
					}
					sql := fmt.Sprintf("DROP DATABASE IF EXISTS \"%s\"; DROP ROLE IF EXISTS \"%s\";", dbName, userName)
					_, err := db.ExecutePostgres(context.Background(), containerTarget, sql)
					if err != nil {
						return err
					}
				} else if engine == "redis" {
					if containerTarget == "" {
						containerTarget = "redis"
					}
					aclArgs := db.BuildRedisDropACL(userName)
					_, err := db.ExecuteRedis(context.Background(), containerTarget, aclArgs...)
					if err != nil {
						return err
					}
				}
				if engine == "redis" {
					fmt.Printf("Dropped user %s (prefix: %s)\n", userName, dbName)
				} else {
					fmt.Printf("Dropped database %s and user %s\n", dbName, userName)
				}
				return nil

			default:
				return fmt.Errorf("unknown db action: %s", action)
			}
		},
	}

	cmd.Flags().BoolVarP(&noExport, "no-export", "n", false, "Do not export credentials to config/env")

	return cmd
}

func writeEnvFile(workDir, engine, containerTarget, dbName, userName, password string) error {
	if workDir == "" {
		workDir = "."
	}
	envDir := filepath.Join(workDir, "config", "env")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return err
	}

	fileName := fmt.Sprintf("%s.%s.env", containerTarget, userName)
	filePath := filepath.Join(envDir, fileName)

	var content string
	switch engine {
	case "mysql":
		port := "3306"
		dbUrl := fmt.Sprintf("mysql://%s:%s@%s:%s/%s", userName, password, containerTarget, port, dbName)
		content = fmt.Sprintf(`# Generated by oops db
DB_ENGINE=mysql
MYSQL_HOST=%s
MYSQL_PORT=%s
MYSQL_DATABASE=%s
MYSQL_USER=%s
MYSQL_PASSWORD=%s
MYSQL_URL=%s
`, containerTarget, port, dbName, userName, password, dbUrl)

	case "postgres":
		port := "5432"
		dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", userName, password, containerTarget, port, dbName)
		content = fmt.Sprintf(`# Generated by oops db
DB_ENGINE=postgres
POSTGRES_HOST=%s
POSTGRES_PORT=%s
POSTGRES_DATABASE=%s
POSTGRES_USER=%s
POSTGRES_PASSWORD=%s
POSTGRES_URL=%s
`, containerTarget, port, dbName, userName, password, dbUrl)

	case "redis":
		port := "6379"
		dbUrl := fmt.Sprintf("redis://%s:%s@%s:%s/0", userName, password, containerTarget, port)
		content = fmt.Sprintf(`# Generated by oops db
DB_ENGINE=redis
REDIS_HOST=%s
REDIS_PORT=%s
REDIS_USER=%s
REDIS_PASSWORD=%s
REDIS_DB=0
REDIS_PREFIX=%s
REDIS_URL=%s
`, containerTarget, port, userName, password, dbName, dbUrl)
	}

	err := os.WriteFile(filePath, []byte(content), 0600)
	if err == nil {
		fmt.Printf("Exported credentials to: %s\n", filePath)
	}
	return err
}

func writePasswdEnvFile(workDir, engine, containerTarget, userName, password string) error {
	if workDir == "" {
		workDir = "."
	}
	envDir := filepath.Join(workDir, "config", "env")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return err
	}

	updatedCount := 0

	// Try to find the associated dbName by scanning existing .env files
	entries, err := os.ReadDir(envDir)
	if err == nil {
		prefix := containerTarget + "."
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".env") {
				content, err := os.ReadFile(filepath.Join(envDir, entry.Name()))
				if err == nil {
					lines := strings.Split(string(content), "\n")
					for _, line := range lines {
						tLine := strings.TrimSpace(line)
						if tLine == "DB_USER="+userName ||
							tLine == "MYSQL_USER="+userName ||
							tLine == "POSTGRES_USER="+userName ||
							tLine == "REDIS_USER="+userName {
							// Found the matching user, extract dbName from filename
							dbName := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), prefix), ".env")
							// Overwrite the original full file
							if err := writeEnvFile(workDir, engine, containerTarget, dbName, userName, password); err == nil {
								updatedCount++
							}
							break // break inner loop, move to next file
						}
					}
				}
			}
		}
	}

	// If we successfully updated at least one file, we are done
	if updatedCount > 0 {
		return nil
	}

	// Fallback if no matching user was found in existing files
	fileName := fmt.Sprintf("%s.%s.env", containerTarget, userName)
	filePath := filepath.Join(envDir, fileName)

	var content string
	switch engine {
	case "mysql":
		content = fmt.Sprintf(`# Generated by oops db passwd (Partial)
DB_ENGINE=mysql
MYSQL_HOST=%s
MYSQL_USER=%s
MYSQL_PASSWORD=%s
`, containerTarget, userName, password)
	case "postgres":
		content = fmt.Sprintf(`# Generated by oops db passwd (Partial)
DB_ENGINE=postgres
POSTGRES_HOST=%s
POSTGRES_USER=%s
POSTGRES_PASSWORD=%s
`, containerTarget, userName, password)
	case "redis":
		content = fmt.Sprintf(`# Generated by oops db passwd (Partial)
DB_ENGINE=redis
REDIS_HOST=%s
REDIS_USER=%s
REDIS_PASSWORD=%s
`, containerTarget, userName, password)
	}

	err = os.WriteFile(filePath, []byte(content), 0600)
	if err == nil {
		fmt.Printf("Exported credentials to: %s\n", filePath)
	}
	return err
}
