package remote

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComposeDependencyInspector inspects compose YAML files for service dependencies.
type ComposeDependencyInspector struct {
	Services      map[string]struct{}
	StackFiles    []string
	ServiceFiles  []string
	ConfigFiles   []string
}

// ResolveRequiredDependencies scans manifests for a given stack and capsule overlay.
func ResolveRequiredDependencies(workDir, stack, remoteName string) (*ComposeDependencyInspector, error) {
	depSet := make(map[string]struct{})
	var manifestFiles []string

	// 1. Stack compose files in root workspace
	if stack != "" {
		stackFile := filepath.Join(workDir, "compose."+stack+".yaml")
		if _, err := os.Stat(stackFile); err == nil {
			manifestFiles = append(manifestFiles, stackFile)
		}
	} else {
		// All non-local, non-example compose.*.yaml
		entries, err := os.ReadDir(workDir)
		if err == nil {
			for _, e := range entries {
				name := e.Name()
				if strings.HasPrefix(name, "compose.") && strings.HasSuffix(name, ".yaml") {
					if !strings.Contains(name, ".local.") && !strings.HasSuffix(name, ".example") {
						manifestFiles = append(manifestFiles, filepath.Join(workDir, name))
					}
				}
			}
		}
	}

	// 2. Remote base compose.yaml from capsule
	capsule := CapsuleDir(workDir, remoteName)
	capsuleCompose := filepath.Join(capsule, "compose.yaml")
	if _, err := os.Stat(capsuleCompose); err == nil {
		manifestFiles = append(manifestFiles, capsuleCompose)
	}

	// 3. Remote machine overrides
	capsuleLocal := filepath.Join(capsule, "compose.local.yaml")
	if _, err := os.Stat(capsuleLocal); err == nil {
		manifestFiles = append(manifestFiles, capsuleLocal)
	}
	if stack != "" {
		capsuleStackLocal := filepath.Join(capsule, "compose."+stack+".local.yaml")
		if _, err := os.Stat(capsuleStackLocal); err == nil {
			manifestFiles = append(manifestFiles, capsuleStackLocal)
		}
	}

	// 4. Parse YAML manifests to find x-oops-services and depends_on
	for _, mPath := range manifestFiles {
		data, err := os.ReadFile(mPath)
		if err != nil {
			continue
		}

		var raw map[string]interface{}
		if err := yaml.Unmarshal(data, &raw); err != nil {
			continue
		}

		// Check x-oops-services
		if v, ok := raw["x-oops-services"]; ok {
			extractServiceNames(v, depSet)
		}

		// Check services: -> container -> depends_on
		if svcs, ok := raw["services"].(map[string]interface{}); ok {
			for _, svcDef := range svcs {
				if svcMap, ok := svcDef.(map[string]interface{}); ok {
					if deps, ok := svcMap["depends_on"]; ok {
						extractServiceNames(deps, depSet)
					}
				}
			}
		}
	}

	inspector := &ComposeDependencyInspector{
		Services: depSet,
	}

	// Filter Stack files to stream
	for _, mf := range manifestFiles {
		rel, err := filepath.Rel(workDir, mf)
		if err == nil && strings.HasPrefix(rel, "compose.") && !strings.Contains(rel, "@") {
			inspector.StackFiles = append(inspector.StackFiles, rel)
		}
	}

	// Locate service templates
	for svc := range depSet {
		svcFile := filepath.Join("services", svc+".yaml")
		if _, err := os.Stat(filepath.Join(workDir, svcFile)); err == nil {
			inspector.ServiceFiles = append(inspector.ServiceFiles, svcFile)
		}

		configDir := filepath.Join("config", svc)
		if fi, err := os.Stat(filepath.Join(workDir, configDir)); err == nil && fi.IsDir() {
			inspector.ConfigFiles = append(inspector.ConfigFiles, configDir)
		}
	}

	return inspector, nil
}

func extractServiceNames(v interface{}, depSet map[string]struct{}) {
	switch val := v.(type) {
	case []interface{}:
		for _, item := range val {
			if s, ok := item.(string); ok {
				clean := strings.TrimSpace(s)
				if clean != "" {
					depSet[clean] = struct{}{}
				}
			}
		}
	case map[string]interface{}:
		for k := range val {
			clean := strings.TrimSpace(k)
			if clean != "" {
				depSet[clean] = struct{}{}
			}
		}
	case string:
		clean := strings.TrimSpace(val)
		if clean != "" {
			depSet[clean] = struct{}{}
		}
	}
}
