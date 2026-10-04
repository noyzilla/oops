package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/noyzilla/oops/internal/db"
	"github.com/spf13/cobra"
)

func newDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db <engine>[:<target>] <action> [args...]",
		Short: "Database provisioning and credentials management (mysql, pg)",
		Long:  "Manages database and user provisioning. Actions: create, passwd, list, drop.",
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
				if len(remainingArgs) < 2 {
					return fmt.Errorf("usage: oops db %s create <db_name> <user_name> [password]", engineSpec)
				}
				dbName := remainingArgs[0]
				userName := remainingArgs[1]
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
					sql := db.BuildMySQLCreateSQL(dbName, userName, password)
					_, err := db.ExecuteMySQL(context.Background(), containerTarget, sql)
					if err != nil {
						return err
					}
				} else if engine == "pg" || engine == "postgres" {
					if containerTarget == "" {
						containerTarget = "postgres"
					}
					sql := db.BuildPostgresCreateSQL(dbName, userName, password)
					_, err := db.ExecutePostgres(context.Background(), containerTarget, sql)
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("unsupported database engine: %s", engine)
				}

				fmt.Printf("Database: %s\nUser: %s\nPassword: %s\n", dbName, userName, password)
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
				} else if engine == "pg" || engine == "postgres" {
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
				} else if engine == "pg" || engine == "postgres" {
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
				} else if engine == "pg" || engine == "postgres" {
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

	return cmd
}
