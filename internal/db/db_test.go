package db_test

import (
	"strings"
	"testing"

	"github.com/noyzilla/oops/internal/db"
)

func TestGeneratePassword(t *testing.T) {
	pass, err := db.GeneratePassword(20)
	if err != nil {
		t.Fatalf("unexpected error generating password: %v", err)
	}
	if len(pass) != 20 {
		t.Errorf("expected 20 characters, got %d (%s)", len(pass), pass)
	}

	// Test distinctness
	pass2, _ := db.GeneratePassword(20)
	if pass == pass2 {
		t.Errorf("expected random passwords to be distinct")
	}
}

func TestBuildMySQLCreateSQL(t *testing.T) {
	sql := db.BuildMySQLCreateSQL("myapp", "Secret123", true)
	if !strings.Contains(sql, "CREATE DATABASE IF NOT EXISTS `myapp`") {
		t.Errorf("missing create database: %s", sql)
	}
	if !strings.Contains(sql, "CREATE USER IF NOT EXISTS 'myapp'@'%' IDENTIFIED BY 'Secret123'") {
		t.Errorf("missing create user: %s", sql)
	}
	if !strings.Contains(sql, "GRANT ALL PRIVILEGES ON `myapp`.* TO 'myapp'@'%'") {
		t.Errorf("missing grant privileges: %s", sql)
	}
	if !strings.Contains(sql, "GRANT ALL PRIVILEGES ON `myapp\\_%`.* TO 'myapp'@'%'") {
		t.Errorf("missing wildcard privileges: %s", sql)
	}
}

func TestBuildPostgresCreateSQL(t *testing.T) {
	sql := db.BuildPostgresCreateSQL("myapp_db", "Secret123", "myapp")
	if !strings.Contains(sql, "CREATE DATABASE \"myapp_db\" OWNER \"myapp_db\"") {
		t.Errorf("missing create database: %s", sql)
	}
	if !strings.Contains(sql, "CREATE ROLE \"myapp_db\" WITH LOGIN PASSWORD 'Secret123'") {
		t.Errorf("missing create role: %s", sql)
	}
	if !strings.Contains(sql, "GRANT ALL PRIVILEGES ON DATABASE \"myapp_db\" TO \"myapp\"") {
		t.Errorf("missing top-user grant: %s", sql)
	}
}

func TestBuildMySQLRepairSQL(t *testing.T) {
	sqlNormal := db.BuildMySQLRepairSQL("myapp", "myapp", "pass123", true, false)
	if !strings.Contains(sqlNormal, "CREATE USER IF NOT EXISTS 'myapp'@'%' IDENTIFIED BY 'pass123'") {
		t.Errorf("missing create user: %s", sqlNormal)
	}
	if !strings.Contains(sqlNormal, "ALTER USER 'myapp'@'%' IDENTIFIED BY 'pass123'") {
		t.Errorf("missing alter user: %s", sqlNormal)
	}
	if !strings.Contains(sqlNormal, "GRANT ALL PRIVILEGES ON `myapp`.* TO 'myapp'@'%'") {
		t.Errorf("missing grant: %s", sqlNormal)
	}

	sqlRO := db.BuildMySQLRepairSQL("myapp", "myapp__ro_app", "pass123", false, true)
	if !strings.Contains(sqlRO, "GRANT SELECT ON `myapp`.* TO 'myapp__ro_app'@'%'") {
		t.Errorf("missing readonly grant: %s", sqlRO)
	}
}

func TestBuildPostgresRepairSQL(t *testing.T) {
	sqlNormal := db.BuildPostgresRepairSQL("myapp_db", "myapp_db", "pass123", "myapp", false)
	if !strings.Contains(sqlNormal, "CREATE ROLE \"myapp_db\" WITH LOGIN PASSWORD 'pass123'") {
		t.Errorf("missing create role: %s", sqlNormal)
	}
	if !strings.Contains(sqlNormal, "ALTER ROLE \"myapp_db\" WITH PASSWORD 'pass123'") {
		t.Errorf("missing alter role: %s", sqlNormal)
	}

	sqlRO := db.BuildPostgresRepairSQL("myapp_db", "myapp__ro_app", "pass123", "", true)
	if !strings.Contains(sqlRO, "GRANT SELECT ON ALL TABLES IN SCHEMA public TO \"myapp__ro_app\"") {
		t.Errorf("missing readonly select: %s", sqlRO)
	}
}
