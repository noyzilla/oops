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
	sql := db.BuildMySQLCreateSQL("myapp_db", "myapp_user", "Secret123")
	if !strings.Contains(sql, "CREATE DATABASE IF NOT EXISTS `myapp_db`") {
		t.Errorf("missing create database: %s", sql)
	}
	if !strings.Contains(sql, "CREATE USER IF NOT EXISTS 'myapp_user'@'%' IDENTIFIED BY 'Secret123'") {
		t.Errorf("missing create user: %s", sql)
	}
	if !strings.Contains(sql, "GRANT ALL PRIVILEGES ON `myapp_db`.* TO 'myapp_user'@'%'") {
		t.Errorf("missing grant privileges: %s", sql)
	}
}

func TestBuildPostgresCreateSQL(t *testing.T) {
	sql := db.BuildPostgresCreateSQL("myapp_db", "myapp_user", "Secret123")
	if !strings.Contains(sql, "CREATE DATABASE \"myapp_db\" OWNER \"myapp_user\"") {
		t.Errorf("missing create database: %s", sql)
	}
	if !strings.Contains(sql, "CREATE ROLE \"myapp_user\" WITH LOGIN PASSWORD 'Secret123'") {
		t.Errorf("missing create role: %s", sql)
	}
}
