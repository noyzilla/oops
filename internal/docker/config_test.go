package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOopsConfigVolumeAndPriority(t *testing.T) {
	dir := t.TempDir()
	yml := "volume:\n  mount_prefixes: []\n"
	if err := os.WriteFile(filepath.Join(dir, "oops.yaml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadOopsConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Volume.MountPrefixes == nil || len(cfg.Volume.MountPrefixes) != 0 {
		t.Fatalf("empty list must be preserved as non-nil, got %#v", cfg.Volume.MountPrefixes)
	}

	empty, err := LoadOopsConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if empty.Volume.MountPrefixes != nil {
		t.Fatal("omitted mount_prefixes must stay nil so defaults apply")
	}
}

func TestLoadOopsConfig_XOops(t *testing.T) {
	dir := t.TempDir()
	yml := `services:
  oops:
    image: ghcr.io/noyzilla/oops:latest
x-oops:
  registries:
    my_org: ghcr.io/my_org
  backups:
    retention: 14d
  dns:
    tld: mytld
    upstreams:
      - 1.1.1.1:53
  volume:
    mount_prefixes: [/mnt/data]
`
	if err := os.WriteFile(filepath.Join(dir, "oops.yaml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadOopsConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Registries["my_org"] != "ghcr.io/my_org" {
		t.Errorf("expected registry alias, got %v", cfg.Registries)
	}
	if cfg.Backups.Retention != "14d" {
		t.Errorf("expected retention 14d, got %s", cfg.Backups.Retention)
	}
	if cfg.DNS.TLD != "mytld" {
		t.Errorf("expected tld mytld, got %s", cfg.DNS.TLD)
	}
	if len(cfg.Volume.MountPrefixes) != 1 || cfg.Volume.MountPrefixes[0] != "/mnt/data" {
		t.Errorf("expected mount_prefixes [/mnt/data], got %v", cfg.Volume.MountPrefixes)
	}
}

func TestLoadOopsConfig_XOopsTreeAndEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	yml := `services:
  oops:
    image: ghcr.io/noyzilla/oops:latest
x-oops:
  secret: base-secret
  db:
    mysql_password: yaml-mysql-pass
    postgres_password: yaml-pg-pass
    redis_password: yaml-redis-pass
  orchestrator:
    stop_timeout: 45s
    healthcheck:
      timeout: 5m
      interval: 2s
  backup:
    dir: ./yaml-backups
    retention: 30d
  dns:
    tld: test-tld
    upstreams:
      - 9.9.9.9:53
`
	if err := os.WriteFile(filepath.Join(dir, "oops.yaml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. Without env overrides: should reflect oops.yaml
	cfg, err := LoadOopsConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Secret != "base-secret" {
		t.Errorf("expected secret 'base-secret', got %q", cfg.Secret)
	}
	if cfg.DB.MySQLPassword != "yaml-mysql-pass" {
		t.Errorf("expected mysql pass 'yaml-mysql-pass', got %q", cfg.DB.MySQLPassword)
	}
	if cfg.DB.PostgresPassword != "yaml-pg-pass" {
		t.Errorf("expected pg pass 'yaml-pg-pass', got %q", cfg.DB.PostgresPassword)
	}
	if cfg.DB.RedisPassword != "yaml-redis-pass" {
		t.Errorf("expected redis pass 'yaml-redis-pass', got %q", cfg.DB.RedisPassword)
	}
	if cfg.Orchestrator.StopTimeout != "45s" {
		t.Errorf("expected stop timeout '45s', got %q", cfg.Orchestrator.StopTimeout)
	}
	if cfg.Orchestrator.Healthcheck.Timeout != "5m" {
		t.Errorf("expected healthcheck timeout '5m', got %q", cfg.Orchestrator.Healthcheck.Timeout)
	}
	if cfg.Orchestrator.Healthcheck.Interval != "2s" {
		t.Errorf("expected healthcheck interval '2s', got %q", cfg.Orchestrator.Healthcheck.Interval)
	}
	if cfg.Backup.Dir != "./yaml-backups" || cfg.Backups.Dir != "./yaml-backups" {
		t.Errorf("expected backup dir './yaml-backups', got %q", cfg.Backup.Dir)
	}
	if cfg.Backup.Retention != "30d" || cfg.Backups.Retention != "30d" {
		t.Errorf("expected retention '30d', got %q", cfg.Backup.Retention)
	}

	// 2. With OOPS_DB_* env overrides: env variables must take precedence
	t.Setenv("OOPS_SECRET", "env-secret-override")
	t.Setenv("OOPS_DB_MYSQL_PASSWORD", "env-mysql-override")
	t.Setenv("OOPS_DB_POSTGRES_PASSWORD", "env-pg-override")
	t.Setenv("OOPS_DB_REDIS_PASSWORD", "env-redis-override")
	t.Setenv("OOPS_STOP_TIMEOUT", "60s")
	t.Setenv("OOPS_HEALTHCHECK_TIMEOUT", "15m")
	t.Setenv("OOPS_BACKUP_RETENTION", "14d")
	t.Setenv("OOPS_DNS_TLD", "env-tld")

	cfgOverridden, err := LoadOopsConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfgOverridden.Secret != "env-secret-override" {
		t.Errorf("expected secret override 'env-secret-override', got %q", cfgOverridden.Secret)
	}
	if cfgOverridden.DB.MySQLPassword != "env-mysql-override" {
		t.Errorf("expected mysql pass override 'env-mysql-override', got %q", cfgOverridden.DB.MySQLPassword)
	}
	if cfgOverridden.DB.PostgresPassword != "env-pg-override" {
		t.Errorf("expected pg pass override 'env-pg-override', got %q", cfgOverridden.DB.PostgresPassword)
	}
	if cfgOverridden.DB.RedisPassword != "env-redis-override" {
		t.Errorf("expected redis pass override 'env-redis-override', got %q", cfgOverridden.DB.RedisPassword)
	}
	if cfgOverridden.Orchestrator.StopTimeout != "60s" {
		t.Errorf("expected stop timeout override '60s', got %q", cfgOverridden.Orchestrator.StopTimeout)
	}
	if cfgOverridden.Orchestrator.Healthcheck.Timeout != "15m" {
		t.Errorf("expected healthcheck timeout override '15m', got %q", cfgOverridden.Orchestrator.Healthcheck.Timeout)
	}
	if cfgOverridden.Backup.Retention != "14d" {
		t.Errorf("expected backup retention override '14d', got %q", cfgOverridden.Backup.Retention)
	}
	if cfgOverridden.DNS.TLD != "env-tld" {
		t.Errorf("expected tld override 'env-tld', got %q", cfgOverridden.DNS.TLD)
	}
}

func TestLoadOopsConfig_NetworkSubnet(t *testing.T) {
	dir := t.TempDir()
	yml := `x-oops:
  network:
    subnet: 10.240.0.0/16
  dns:
    tld: b9tpe
`
	if err := os.WriteFile(filepath.Join(dir, "oops.yaml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadOopsConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Network.Subnet != "10.240.0.0/16" {
		t.Errorf("expected subnet 10.240.0.0/16, got %q", cfg.Network.Subnet)
	}
	if cfg.DNS.TLD != "b9tpe" {
		t.Errorf("expected tld b9tpe, got %q", cfg.DNS.TLD)
	}

	// Test env override
	t.Setenv("OOPS_NETWORK_SUBNET", "10.50.0.0/16")
	cfgEnv, err := LoadOopsConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfgEnv.Network.Subnet != "10.50.0.0/16" {
		t.Errorf("expected env override 10.50.0.0/16, got %q", cfgEnv.Network.Subnet)
	}
}

func TestValidateNetworkSubnet(t *testing.T) {
	valid := []string{
		"",
		"10.240.0.0/16",
		"10.50.0.0/16",
		"192.168.1.0/24",
		"172.20.0.0/16",
	}
	for _, cidr := range valid {
		if err := ValidateNetworkSubnet(cidr); err != nil {
			t.Errorf("expected %q to be valid, got: %v", cidr, err)
		}
	}

	invalid := []string{
		"not-a-cidr",
		"127.0.0.0/8",      // loopback blocked
		"127.240.0.0/16",   // loopback blocked
		"2001:db8::/32",    // IPv6 blocked
		"10.0.0.0/6",       // prefix < /8
		"10.0.0.0/30",      // prefix > /28
	}
	for _, cidr := range invalid {
		if err := ValidateNetworkSubnet(cidr); err == nil {
			t.Errorf("expected %q to be invalid, but validation passed", cidr)
		}
	}
}

func TestIsSubnetContained(t *testing.T) {
	parent := "10.240.0.0/16"

	// Valid children
	contained := []string{
		"10.240.0.0/16",
		"10.240.1.0/24",
		"10.240.2.0/24",
		"10.240.254.0/24",
	}
	for _, child := range contained {
		ok, err := IsSubnetContained(parent, child)
		if err != nil || !ok {
			t.Errorf("expected %q to be contained in %q, got ok=%v, err=%v", child, parent, ok, err)
		}
	}

	// Invalid / outside children
	outside := []string{
		"10.241.0.0/16",
		"172.20.0.0/16",
		"10.0.0.0/8", // larger than parent
		"192.168.1.0/24",
	}
	for _, child := range outside {
		ok, err := IsSubnetContained(parent, child)
		if err != nil {
			t.Errorf("unexpected error for %q: %v", child, err)
		}
		if ok {
			t.Errorf("expected %q NOT to be contained in %q", child, parent)
		}
	}
}
