package db

import (
	"reflect"
	"testing"
)

func TestBuildRedisCreateACL(t *testing.T) {
	args := BuildRedisCreateACL("myapp", "secret123", "myapp")
	expected := []string{
		"ACL", "SETUSER", "myapp",
		"on", ">secret123",
		"~myapp:*",
		"&*",
		"+@all",
		"-@dangerous",
		"+info",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Errorf("BuildRedisCreateACL() = %v, want %v", args, expected)
	}
}

func TestBuildRedisReadonlyACL(t *testing.T) {
	args := BuildRedisReadonlyACL("myapp__ro_worker", "secret123", "myapp")
	expected := []string{
		"ACL", "SETUSER", "myapp__ro_worker",
		"on", ">secret123",
		"~myapp:*",
		"&*",
		"+@read",
		"+info",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Errorf("BuildRedisReadonlyACL() = %v, want %v", args, expected)
	}
}

func TestBuildRedisPasswdACL(t *testing.T) {
	args := BuildRedisPasswdACL("myapp", "newpass456")
	expected := []string{
		"ACL", "SETUSER", "myapp",
		"on",
		"resetpass",
		">newpass456",
		"+info",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Errorf("BuildRedisPasswdACL() = %v, want %v", args, expected)
	}
}

func TestBuildRedisDropACL(t *testing.T) {
	args := BuildRedisDropACL("myapp")
	expected := []string{
		"ACL", "DELUSER", "myapp",
	}

	if !reflect.DeepEqual(args, expected) {
		t.Errorf("BuildRedisDropACL() = %v, want %v", args, expected)
	}
}

