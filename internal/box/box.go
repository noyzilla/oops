package box

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandHome expands leading ~ in paths to user's home directory.
func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// CanonicalPath returns absolute and symlink-evaluated path.
func CanonicalPath(path string) string {
	expanded := ExpandHome(path)
	abs, err := filepath.Abs(expanded)
	if err != nil {
		abs = expanded
	}
	eval, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return eval
	}
	return abs
}

// GetActiveBox reads the active workspace path from ~/.oops/active_box.
func GetActiveBox() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", nil
	}
	activeFile := filepath.Join(home, ".oops", "active_box")
	data, err := os.ReadFile(activeFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	path := strings.TrimSpace(string(data))
	if path == "" {
		return "", nil
	}
	return ExpandHome(path), nil
}

// SetActiveBox writes the active workspace path to ~/.oops/active_box.
func SetActiveBox(path string) error {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	dir := filepath.Join(home, ".oops")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	activeFile := filepath.Join(dir, "active_box")
	canonical := CanonicalPath(path)
	return os.WriteFile(activeFile, []byte(canonical+"\n"), 0644)
}

// CreateWorkspace provisions a new Oopsbox workspace at targetPath from the release blueprint.
func CreateWorkspace(targetPath string) (string, error) {
	canonical := CanonicalPath(targetPath)
	fmt.Printf("==> Creating Oopsbox workspace at: %s\n", canonical)

	if err := os.MkdirAll(canonical, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", canonical, err)
	}

	// 1. Download & extract blueprint
	fmt.Println("==> Fetching latest Oopsbox blueprint...")
	if err := FetchAndExtractBlueprint(canonical); err != nil {
		return "", fmt.Errorf("failed to fetch blueprint: %w", err)
	}

	// 2. Initialize .env with strong credentials
	if err := InitWorkspaceEnv(canonical); err != nil {
		return "", fmt.Errorf("failed to initialize .env credentials: %w", err)
	}

	// 3. Initialize oopsbox.yml if not exists
	cfgPath := filepath.Join(canonical, "oopsbox.yml")
	exampleCfg := filepath.Join(canonical, "oopsbox.yml.example")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if data, err := os.ReadFile(exampleCfg); err == nil {
			if err := os.WriteFile(cfgPath, data, 0644); err != nil {
				return "", fmt.Errorf("failed to create oopsbox.yml: %w", err)
			}
			fmt.Println("==> Initialized oopsbox.yml from template.")
		}
	}

	// 4. Set as active box
	if err := SetActiveBox(canonical); err != nil {
		return "", fmt.Errorf("failed to set active box: %w", err)
	}
	fmt.Printf("==> Workspace active: %s\n", canonical)

	return canonical, nil
}

// InitWorkspaceEnv generates secure random credentials for .env in the target workspace.
func InitWorkspaceEnv(workDir string) error {
	envFile := filepath.Join(workDir, ".env")
	envExampleFile := filepath.Join(workDir, ".env.example")

	secret, err := GenerateRandomSecret(48)
	if err != nil {
		return err
	}
	dbPass, err := GenerateRandomSecret(32)
	if err != nil {
		return err
	}

	// If .env already exists, check if OOPS_SECRET is set
	if data, err := os.ReadFile(envFile); err == nil {
		content := string(data)
		var updated bool
		if !strings.Contains(content, "OOPS_SECRET=") || strings.Contains(content, "OOPS_SECRET=\n") || strings.Contains(content, "OOPS_SECRET=\"\"") {
			content += fmt.Sprintf("\nOOPS_SECRET=%s\n", secret)
			updated = true
		}
		if !strings.Contains(content, "MYSQL_ROOT_PASSWORD=") {
			content += fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s\n", dbPass)
			updated = true
		}
		if !strings.Contains(content, "POSTGRES_PASSWORD=") {
			content += fmt.Sprintf("POSTGRES_PASSWORD=%s\n", dbPass)
			updated = true
		}
		if !strings.Contains(content, "REDIS_PASSWORD=") {
			content += fmt.Sprintf("REDIS_PASSWORD=%s\n", dbPass)
			updated = true
		}
		if updated {
			return os.WriteFile(envFile, []byte(content), 0600)
		}
		return nil
	}

	// If .env.example exists, read and interpolate
	var envContent string
	if data, err := os.ReadFile(envExampleFile); err == nil {
		template := string(data)
		template = strings.ReplaceAll(template, "OOPS_SECRET=", fmt.Sprintf("OOPS_SECRET=%s", secret))
		template = strings.ReplaceAll(template, "MYSQL_ROOT_PASSWORD=secret", fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", dbPass))
		template = strings.ReplaceAll(template, "POSTGRES_PASSWORD=secret", fmt.Sprintf("POSTGRES_PASSWORD=%s", dbPass))
		template = strings.ReplaceAll(template, "REDIS_PASSWORD=secret", fmt.Sprintf("REDIS_PASSWORD=%s", dbPass))
		if !strings.Contains(template, "OOPS_SECRET=") {
			template = fmt.Sprintf("OOPS_SECRET=%s\n", secret) + template
		}
		envContent = template
	} else {
		envContent = fmt.Sprintf(`OOPS_SECRET=%s
MYSQL_ROOT_PASSWORD=%s
POSTGRES_PASSWORD=%s
REDIS_PASSWORD=%s
`, secret, dbPass, dbPass, dbPass)
	}

	fmt.Println("==> Generated secure random credentials in .env")
	return os.WriteFile(envFile, []byte(envContent), 0600)
}

// HasValidStacks checks if a directory contains a stacks/ folder or compose files.
func HasValidStacks(dir string) bool {
	if fi, err := os.Stat(filepath.Join(dir, "stacks")); err == nil && fi.IsDir() {
		return true
	}
	for _, name := range []string{"compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml"} {
		if fi, err := os.Stat(filepath.Join(dir, name)); err == nil && !fi.IsDir() {
			return true
		}
	}
	return false
}

// DiscoverWorkspaces returns a list of candidate Oopsbox workspaces.
func DiscoverWorkspaces() []string {
	var candidates []string
	home, _ := os.UserHomeDir()

	if home != "" {
		candidates = append(candidates, filepath.Join(home, "oopsbox"))
		// Search ~/Workspaces/*/oopsbox
		pattern := filepath.Join(home, "Workspaces", "*", "oopsbox")
		if matches, err := filepath.Glob(pattern); err == nil {
			candidates = append(candidates, matches...)
		}
	}
	candidates = append(candidates, "/opt/oopsbox")

	if active, err := GetActiveBox(); err == nil && active != "" {
		candidates = append([]string{active}, candidates...)
	}

	// Deduplicate and filter existing valid directories
	seen := make(map[string]bool)
	var valid []string
	for _, c := range candidates {
		canon := CanonicalPath(c)
		if seen[canon] {
			continue
		}
		seen[canon] = true
		if HasValidStacks(canon) {
			valid = append(valid, canon)
		}
	}
	return valid
}
