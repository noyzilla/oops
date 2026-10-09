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
		Short: "Database provisioning and credentials management for mysql or postgres",
		Long:  "Manages database and user provisioning for supported engines: mysql, postgres. Actions: create, readonly, passwd, list, drop.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
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
					return fmt.Errorf("usage: oops db %s create <db_name> [password]", engineSpec)
				}
				dbName := remainingArgs[0]
				userName := dbName
				
				dbParts := strings.Split(dbName, "_")
				if len(dbParts) > 2 {
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
				} else {
					return fmt.Errorf("unsupported database engine: %s", engine)
				}

				fmt.Printf("Database: %s\nUser: %s\nPassword: %s\n", dbName, userName, password)
				
				if !noExport {
					if err := writeEnvFile(engine, containerTarget, dbName, userName, password); err != nil {
						fmt.Printf("Warning: failed to export env file: %v\n", err)
					}
				}
				
				return nil

			case "readonly":
				if len(remainingArgs) < 2 {
					return fmt.Errorf("usage: oops db %s readonly <db_name> <app_name> [password]", engineSpec)
				}
				dbName := remainingArgs[0]
				appName := remainingArgs[1]
				
				dbParts := strings.Split(dbName, "_")
				if len(dbParts) > 2 {
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
				} else {
					return fmt.Errorf("unsupported database engine: %s", engine)
				}

				fmt.Printf("Readonly User: %s\nTarget DB: %s\nPassword: %s\n", userName, dbName, password)
				
				if !noExport {
					if err := writeEnvFile(engine, containerTarget, dbName, userName, password); err != nil {
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
				} else {
					return fmt.Errorf("unsupported database engine: %s", engine)
				}

				fmt.Printf("User: %s\nNew Password: %s\n", userName, newPassword)

				if !noExport {
					if err := writePasswdEnvFile(engine, containerTarget, userName, newPassword); err != nil {
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
				}
				return nil

			case "drop":
				if len(remainingArgs) < 2 {
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
				}
				fmt.Printf("Dropped database %s and user %s\n", dbName, userName)
				return nil

			default:
				return fmt.Errorf("unknown db action: %s", action)
			}
		},
	}

	cmd.Flags().BoolVarP(&noExport, "no-export", "n", false, "Do not export credentials to config/env")

	return cmd
}

func writeEnvFile(engine, containerTarget, dbName, userName, password string) error {
	envDir := filepath.Join("config", "env")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return err
	}

	fileName := fmt.Sprintf("%s.%s.env", containerTarget, userName)
	filePath := filepath.Join(envDir, fileName)

	var port string
	var dbUrl string
	if engine == "mysql" {
		port = "3306"
		dbUrl = fmt.Sprintf("mysql://%s:%s@%s:%s/%s", userName, password, containerTarget, port, dbName)
	} else if engine == "postgres" {
		port = "5432"
		dbUrl = fmt.Sprintf("postgres://%s:%s@%s:%s/%s", userName, password, containerTarget, port, dbName)
	}

	content := fmt.Sprintf(`# Generated by oops db
DB_ENGINE=%s
DB_HOST=%s
DB_PORT=%s
DB_NAME=%s
DB_USER=%s
DB_PASSWORD=%s
DB_URL=%s
DATABASE_URL=%s
`, engine, containerTarget, port, dbName, userName, password, dbUrl, dbUrl)

	err := os.WriteFile(filePath, []byte(content), 0600)
	if err == nil {
		fmt.Printf("Exported credentials to: %s\n", filePath)
	}
	return err
}

func writePasswdEnvFile(engine, containerTarget, userName, password string) error {
	envDir := filepath.Join("config", "env")
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
						if strings.TrimSpace(line) == "DB_USER="+userName {
							// Found the matching user, extract dbName from filename
							dbName := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), prefix), ".env")
							// Overwrite the original full file
							if err := writeEnvFile(engine, containerTarget, dbName, userName, password); err == nil {
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

	// Fallback if no matching DB_USER was found in existing files
	fileName := fmt.Sprintf("%s.%s.env", containerTarget, userName)
	filePath := filepath.Join(envDir, fileName)

	content := fmt.Sprintf(`# Generated by oops db passwd (Partial)
DB_ENGINE=%s
DB_HOST=%s
DB_USER=%s
DB_PASSWORD=%s
`, engine, containerTarget, userName, password)

	err = os.WriteFile(filePath, []byte(content), 0600)
	if err == nil {
		fmt.Printf("Exported credentials to: %s\n", filePath)
	}
	return err
}
