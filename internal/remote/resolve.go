package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComposeDependencyInspector inspects compose YAML files for service dependencies.
type ComposeDependencyInspector struct {
	Services          map[string]struct{}
	StackFiles        []string
	CapsuleStackFiles []string
	ServiceFiles      []string
	ConfigFiles       []string
	RequiredStacks    []string
}

// ParseStackTargets splits a comma-separated or single stack argument into clean stack names.
func ParseStackTargets(stack string) []string {
	if strings.TrimSpace(stack) == "" {
		return nil
	}
	parts := strings.Split(stack, ",")
	var result []string
	seen := make(map[string]bool)
	for _, p := range parts {
		clean := strings.TrimSpace(p)
		clean = strings.TrimPrefix(clean, "/")
		clean = strings.TrimSpace(clean)
		if clean != "" && !seen[clean] {
			seen[clean] = true
			result = append(result, clean)
		}
	}
	return result
}

type stackManifestInfo struct {
	name        string
	isCapsule   bool
	path        string
	definedSvcs map[string]struct{}
	rawDeps     map[string]struct{}
}

func parseStackManifest(filePath string, name string, isCapsule bool) *stackManifestInfo {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	info := &stackManifestInfo{
		name:        name,
		isCapsule:   isCapsule,
		path:        filePath,
		definedSvcs: make(map[string]struct{}),
		rawDeps:     make(map[string]struct{}),
	}
	if v, ok := raw["x-oops-services"]; ok {
		extractServiceNames(v, info.rawDeps)
	}
	if v, ok := raw["x-oops-depends"]; ok {
		extractServiceNames(v, info.rawDeps)
	}
	if v, ok := raw["x-oops-groups"]; ok {
		extractGroupServiceNames(v, info.rawDeps)
	}
	if svcs, ok := raw["services"].(map[string]interface{}); ok {
		for sName, sDef := range svcs {
			clean := strings.TrimSpace(sName)
			if clean != "" {
				info.definedSvcs[clean] = struct{}{}
			}
			if sMap, ok := sDef.(map[string]interface{}); ok {
				if deps, ok := sMap["depends_on"]; ok {
					extractServiceNames(deps, info.rawDeps)
				}
			}
		}
	}
	return info
}

// ResolveRequiredDependencies scans manifests for a given stack target and capsule overlay.
func ResolveRequiredDependencies(workDir, stack, remoteName string) (*ComposeDependencyInspector, error) {
	capsule := CapsuleDir(workDir, remoteName)
	allStacks := make(map[string]*stackManifestInfo)

	// 1. Scan root workspace for compose.<name>.yaml
	if entries, err := os.ReadDir(workDir); err == nil {
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, "compose.") && strings.HasSuffix(name, ".yaml") {
				if !strings.Contains(name, ".local.") && !strings.HasSuffix(name, ".example") {
					stackName := strings.TrimSuffix(strings.TrimPrefix(name, "compose."), ".yaml")
					if stackName != "" {
						if info := parseStackManifest(filepath.Join(workDir, name), stackName, false); info != nil {
							allStacks[stackName] = info
						}
					}
				}
			}
		}
	}

	// 2. Scan capsule for compose.<name>.yaml (Case 3: capsule overrides stack manifest)
	if entries, err := os.ReadDir(capsule); err == nil {
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, "compose.") && strings.HasSuffix(name, ".yaml") {
				if !strings.Contains(name, ".local.") && !strings.HasSuffix(name, ".example") {
					stackName := strings.TrimSuffix(strings.TrimPrefix(name, "compose."), ".yaml")
					if stackName != "" {
						if info := parseStackManifest(filepath.Join(capsule, name), stackName, true); info != nil {
							allStacks[stackName] = info
						}
					}
				}
			}
		}
	}

	// 3. Determine required stacks starting from target stack(s)
	targets := ParseStackTargets(stack)
	pushedStacks := make(map[string]bool)
	requiredStacksMap := make(map[string]bool)
	var stackQueue []string

	if len(targets) > 0 {
		for _, t := range targets {
			pushedStacks[t] = true
			requiredStacksMap[t] = true
			stackQueue = append(stackQueue, t)
		}
	} else {
		// When no stack target is specified, scope strictly to stacks defined in capsule.
		for sName, sInfo := range allStacks {
			if sInfo.isCapsule {
				pushedStacks[sName] = true
				requiredStacksMap[sName] = true
				stackQueue = append(stackQueue, sName)
			}
		}
	}

	// Traverse cross-stack dependencies (with Strict Remote Boundary Check)
	for len(stackQueue) > 0 {
		curr := stackQueue[0]
		stackQueue = stackQueue[1:]

		sInfo, ok := allStacks[curr]
		if !ok {
			continue
		}

		for dep := range sInfo.rawDeps {
			var depStack string
			if strings.HasPrefix(dep, "/") {
				trimmed := strings.TrimPrefix(dep, "/")
				parts := strings.Split(trimmed, "/")
				depStack = parts[0]
			} else {
				// Bare service name: if not defined in current stack, check other stacks
				if _, isLocal := sInfo.definedSvcs[dep]; !isLocal {
					for otherName, otherInfo := range allStacks {
						if _, hasSvc := otherInfo.definedSvcs[dep]; hasSvc {
							depStack = otherName
							break
						}
					}
				}
			}

			if depStack != "" && depStack != curr {
				if !requiredStacksMap[depStack] {
					otherInfo, exists := allStacks[depStack]
					if !exists {
						continue
					}

					// Strict Remote Boundary Check:
					// If depStack is not in pushed targets, verify it exists in remote capsule.
					// If it only exists in root workspace and not in capsule -> ERROR!
					if len(pushedStacks) > 0 && !pushedStacks[depStack] {
						if !otherInfo.isCapsule {
							return nil, fmt.Errorf("stack '/%s' depends on '%s', but stack '/%s' is not in push targets and does not exist in capsule '/@%s'. Hint: push '/%s,/%s' or provide '/@%s/compose.%s.yaml'",
								curr, dep, depStack, remoteName, curr, depStack, remoteName, depStack)
						}
					}

					requiredStacksMap[depStack] = true
					stackQueue = append(stackQueue, depStack)
				}
			}
		}
	}

	definedServices := make(map[string]struct{})
	depSet := make(map[string]struct{})

	// 4. Record services from remote capsule compose.yaml, compose.local.yaml, and compose.<stack>.local.yaml
	capsuleManifests := []string{
		filepath.Join(capsule, "compose.yaml"),
		filepath.Join(capsule, "compose.local.yaml"),
	}
	for _, t := range targets {
		capsuleManifests = append(capsuleManifests, filepath.Join(capsule, "compose."+t+".local.yaml"))
	}
	for _, mPath := range capsuleManifests {
		if data, err := os.ReadFile(mPath); err == nil {
			var raw map[string]interface{}
			if err := yaml.Unmarshal(data, &raw); err == nil {
				if v, ok := raw["x-oops-services"]; ok {
					extractServiceNames(v, depSet)
				}
				if v, ok := raw["x-oops-depends"]; ok {
					extractServiceNames(v, depSet)
				}
				if v, ok := raw["x-oops-groups"]; ok {
					extractGroupServiceNames(v, depSet)
				}
				if svcs, ok := raw["services"].(map[string]interface{}); ok {
					for sName, sDef := range svcs {
						clean := strings.TrimSpace(sName)
						if clean != "" {
							definedServices[clean] = struct{}{}
						}
						if sMap, ok := sDef.(map[string]interface{}); ok {
							if deps, ok := sMap["depends_on"]; ok {
								extractServiceNames(deps, depSet)
							}
						}
					}
				}
			}
		}
	}

	// 5. Remote capsule service templates (/@<remote>/services/*.yaml)
	capsuleServicesDir := filepath.Join(capsule, "services")
	if entries, err := os.ReadDir(capsuleServicesDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
				baseName := strings.TrimSuffix(e.Name(), ".yaml")
				definedServices[baseName] = struct{}{}
				for _, sName := range parseServiceTemplateNames(filepath.Join(capsuleServicesDir, e.Name())) {
					definedServices[sName] = struct{}{}
				}
			}
		}
	}

	inspector := &ComposeDependencyInspector{
		Services: depSet,
	}

	var reqStacksList []string
	for sName := range requiredStacksMap {
		reqStacksList = append(reqStacksList, sName)
	}
	sort.Strings(reqStacksList)
	inspector.RequiredStacks = reqStacksList

	// 6. Assemble StackFiles (root workspace) vs CapsuleStackFiles (capsule override)
	for _, sName := range reqStacksList {
		sInfo, ok := allStacks[sName]
		if !ok {
			continue
		}
		for s := range sInfo.definedSvcs {
			definedServices[s] = struct{}{}
		}
		for d := range sInfo.rawDeps {
			depSet[d] = struct{}{}
		}

		if sInfo.isCapsule {
			// Case 3: Defined in capsule -> streamed from capsule overlay, NOT from root workspace!
			inspector.CapsuleStackFiles = append(inspector.CapsuleStackFiles, "compose."+sName+".yaml")
		} else {
			// Case 2: Defined in root workspace -> streamed in Step 1
			inspector.StackFiles = append(inspector.StackFiles, "compose."+sName+".yaml")
		}
	}

	// 7. Locate service templates (Case 1 & 2: Local & Remote Precedence)
	templateMap := mapWorkspaceServiceTemplates(filepath.Join(workDir, "services"))
	seenServiceFiles := make(map[string]bool)
	seenConfigDirs := make(map[string]bool)

	for dep := range depSet {
		cleanDep := dep
		if strings.HasPrefix(cleanDep, "/") {
			parts := strings.Split(strings.TrimPrefix(cleanDep, "/"), "/")
			if len(parts) == 2 {
				cleanDep = parts[1]
			} else {
				continue
			}
		}

		// If service is already declared in any required stack or capsule, skip template!
		if _, isDefined := definedServices[cleanDep]; isDefined {
			continue
		}

		templateBase := cleanDep
		if mapped, ok := templateMap[cleanDep]; ok {
			templateBase = mapped
		}
		if _, isDefined := definedServices[templateBase]; isDefined {
			continue
		}

		svcFile := filepath.Join("services", templateBase+".yaml")
		if !seenServiceFiles[svcFile] {
			if _, err := os.Stat(filepath.Join(workDir, svcFile)); err == nil {
				seenServiceFiles[svcFile] = true
				inspector.ServiceFiles = append(inspector.ServiceFiles, svcFile)
			}
		}

		configDir := filepath.Join("config", templateBase)
		if !seenConfigDirs[configDir] {
			if fi, err := os.Stat(filepath.Join(workDir, configDir)); err == nil && fi.IsDir() {
				seenConfigDirs[configDir] = true
				inspector.ConfigFiles = append(inspector.ConfigFiles, configDir)
			}
		}
	}

	sort.Strings(inspector.ServiceFiles)
	sort.Strings(inspector.ConfigFiles)

	return inspector, nil
}

func parseServiceTemplateNames(filePath string) []string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	svcs, ok := raw["services"].(map[string]interface{})
	if !ok {
		return nil
	}
	var names []string
	for sName := range svcs {
		clean := strings.TrimSpace(sName)
		if clean != "" {
			names = append(names, clean)
		}
	}
	return names
}

func mapWorkspaceServiceTemplates(servicesDir string) map[string]string {
	result := make(map[string]string)
	entries, err := os.ReadDir(servicesDir)
	if err != nil {
		return result
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		baseName := strings.TrimSuffix(e.Name(), ".yaml")
		result[baseName] = baseName
		for _, sName := range parseServiceTemplateNames(filepath.Join(servicesDir, e.Name())) {
			if _, exists := result[sName]; !exists {
				result[sName] = baseName
			}
		}
	}
	return result
}

func extractGroupServiceNames(v interface{}, depSet map[string]struct{}) {
	groups, ok := v.(map[string]interface{})
	if !ok {
		return
	}
	for _, groupMembers := range groups {
		extractServiceNames(groupMembers, depSet)
	}
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
