package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComposeDependencyInspector inspects compose YAML files for stack and service dependencies.
type ComposeDependencyInspector struct {
	Services          map[string]struct{}
	StackFiles        []string
	CapsuleStackFiles []string
	ServiceFiles      []string
	ConfigFiles       []string
	RequiredStacks    []string
	Warnings          []string
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
		// If someone passed /db/mysql, stack is db
		if idx := strings.Index(clean, "/"); idx != -1 {
			clean = clean[:idx]
		}
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
	xStack      []string
	rawDeps     map[string]struct{}
	dependsOn   map[string]struct{}
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
		dependsOn:   make(map[string]struct{}),
	}
	for _, key := range []string{"x-oops-stack", "x-oops-stacks"} {
		if v, ok := raw[key]; ok {
			temp := make(map[string]struct{})
			extractServiceNames(v, temp)
			for k := range temp {
				info.xStack = append(info.xStack, k)
				info.rawDeps[k] = struct{}{}
			}
		}
	}
	if v, ok := raw["x-oops-services"]; ok {
		extractServiceNames(v, info.rawDeps)
	}
	if v, ok := raw["x-oops-depends"]; ok {
		extractServiceNames(v, info.rawDeps)
		extractServiceNames(v, info.dependsOn)
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
					extractServiceNames(deps, info.dependsOn)
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

	// Parse capsule compose.yaml and compose.local.yaml first to know capsule's x-oops-stack entries
	capsuleXStackItems := make(map[string]bool)
	capsuleDefinedServices := make(map[string]struct{})
	capsuleDepSet := make(map[string]struct{})

	baseCapsuleManifests := []string{
		filepath.Join(capsule, "compose.yaml"),
		filepath.Join(capsule, "compose.local.yaml"),
	}
	for _, mPath := range baseCapsuleManifests {
		if data, err := os.ReadFile(mPath); err == nil {
			var raw map[string]interface{}
			if err := yaml.Unmarshal(data, &raw); err == nil {
				for _, key := range []string{"x-oops-stack", "x-oops-stacks"} {
					if v, ok := raw[key]; ok {
						temp := make(map[string]struct{})
						extractServiceNames(v, temp)
						for k := range temp {
							capsuleXStackItems[k] = true
							capsuleDepSet[k] = struct{}{}
						}
					}
				}
				if v, ok := raw["x-oops-services"]; ok {
					extractServiceNames(v, capsuleDepSet)
				}
				if v, ok := raw["x-oops-depends"]; ok {
					extractServiceNames(v, capsuleDepSet)
				}
				if svcs, ok := raw["services"].(map[string]interface{}); ok {
					for sName, sDef := range svcs {
						clean := strings.TrimSpace(sName)
						if clean != "" {
							capsuleDefinedServices[clean] = struct{}{}
						}
						if sMap, ok := sDef.(map[string]interface{}); ok {
							if deps, ok := sMap["depends_on"]; ok {
								extractServiceNames(deps, capsuleDepSet)
							}
						}
					}
				}
			}
		}
	}

	// Helper to resolve which stack a reference ("/<stack>", "/<stack>/<service>", or bare service) belongs to
	resolveStackForRef := func(ref string, currentStack string) (string, string) {
		if strings.HasPrefix(ref, "/") {
			trimmed := strings.TrimPrefix(ref, "/")
			parts := strings.SplitN(trimmed, "/", 2)
			sName := parts[0]
			srvName := ""
			if len(parts) == 2 {
				srvName = parts[1]
			}
			return sName, srvName
		}
		if currentStack != "" {
			if sInfo, ok := allStacks[currentStack]; ok {
				if _, isLocal := sInfo.definedSvcs[ref]; isLocal {
					return currentStack, ref
				}
			}
		}
		// Check preferred stacks first (db, edge, storage, tools)
		for _, pref := range []string{"db", "edge", "storage", "tools"} {
			if sInfo, ok := allStacks[pref]; ok {
				if _, has := sInfo.definedSvcs[ref]; has {
					return pref, ref
				}
			}
		}
		for otherName, otherInfo := range allStacks {
			if _, has := otherInfo.definedSvcs[ref]; has {
				return otherName, ref
			}
		}
		return "", ref
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
		// When no stack target is specified, scope strictly to:
		// 1) Stacks defined directly in capsule (compose.<stack>.yaml)
		// 2) Stacks referenced in capsule compose.yaml (via x-oops-stack, etc.)
		for sName, sInfo := range allStacks {
			if sInfo.isCapsule {
				pushedStacks[sName] = true
				requiredStacksMap[sName] = true
				stackQueue = append(stackQueue, sName)
			}
		}
		for item := range capsuleXStackItems {
			refStack, _ := resolveStackForRef(item, "")
			if refStack != "" {
				if _, exists := allStacks[refStack]; exists && !requiredStacksMap[refStack] {
					pushedStacks[refStack] = true
					requiredStacksMap[refStack] = true
					stackQueue = append(stackQueue, refStack)
				}
			}
		}
	}

	var warnings []string
	seenWarnings := make(map[string]bool)
	addWarning := func(msg string) {
		if !seenWarnings[msg] {
			seenWarnings[msg] = true
			warnings = append(warnings, msg)
		}
	}

	// Helper to check if a dependency stack/service is declared in capsule compose.yaml x-oops-stack or parent stack x-oops-stack
	isDeclaredInCapsuleXStack := func(depStack, depSvc string, parentInfo *stackManifestInfo) bool {
		// Check if declared in parent stack's own x-oops-stack
		if parentInfo != nil {
			for _, entry := range parentInfo.xStack {
				if entry == "/"+depStack || entry == "/"+depStack+"/"+depSvc || entry == depSvc {
					return true
				}
			}
		}
		// Check if declared in capsule compose.yaml x-oops-stack
		for entry := range capsuleXStackItems {
			if entry == "/"+depStack || entry == "/"+depStack+"/"+depSvc || entry == depSvc {
				return true
			}
		}
		return false
	}

	// Traverse cross-stack dependencies (with Strict Remote Boundary Check & depends_on Warnings)
	for len(stackQueue) > 0 {
		curr := stackQueue[0]
		stackQueue = stackQueue[1:]

		sInfo, ok := allStacks[curr]
		if !ok {
			continue
		}

		// Check depends_on entries for undeclared x-oops-stack warnings
		for dep := range sInfo.dependsOn {
			if _, isLocal := sInfo.definedSvcs[dep]; isLocal {
				// If stack has x-oops-stack, check if local dependency is included
				if len(sInfo.xStack) > 0 {
					foundLocal := false
					for _, xs := range sInfo.xStack {
						if xs == dep || xs == "/"+curr+"/"+dep {
							foundLocal = true
							break
						}
					}
					if !foundLocal {
						addWarning(fmt.Sprintf("stack '/%s' service depends_on '%s', which is omitted from x-oops-stack in compose.%s.yaml", curr, dep, curr))
					}
				}
				continue
			}
			if _, inCap := capsuleDefinedServices[dep]; inCap {
				continue
			}

			depStack, depSvc := resolveStackForRef(dep, curr)
			if depStack != "" && depStack != curr {
				if !isDeclaredInCapsuleXStack(depStack, depSvc, sInfo) {
					targetRef := "/" + depStack
					if depSvc != "" {
						targetRef = "/" + depStack + "/" + depSvc
					}
					addWarning(fmt.Sprintf("stack '/%s' depends on '%s' (%s), which is not declared in x-oops-stack of compose.%s.yaml or /@%s/compose.yaml", curr, dep, targetRef, curr, remoteName))
				}
			}
		}

		for dep := range sInfo.rawDeps {
			if _, isLocal := sInfo.definedSvcs[dep]; isLocal {
				continue
			}
			depStack, _ := resolveStackForRef(dep, curr)

			if depStack != "" && depStack != curr {
				if !requiredStacksMap[depStack] {
					otherInfo, exists := allStacks[depStack]
					if !exists {
						continue
					}

					// Strict Remote Boundary Check when explicit stack target was given:
					// If depStack is not in pushed targets and not in capsule and not in capsule's x-oops-stack -> ERROR!
					if len(targets) > 0 && !pushedStacks[depStack] {
						if !otherInfo.isCapsule && !isDeclaredInCapsuleXStack(depStack, "", sInfo) {
							return nil, fmt.Errorf("stack '/%s' depends on '%s', but stack '/%s' is not in push targets and does not exist in capsule '/@%s'. Hint: push '/%s,/%s' or declare '/%s' in x-oops-stack",
								curr, dep, depStack, remoteName, curr, depStack, depStack)
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

	for s := range capsuleDefinedServices {
		definedServices[s] = struct{}{}
	}
	for d := range capsuleDepSet {
		depSet[d] = struct{}{}
	}

	// 4. Record services from remote capsule compose.<stack>.local.yaml
	for _, t := range targets {
		mPath := filepath.Join(capsule, "compose."+t+".local.yaml")
		if data, err := os.ReadFile(mPath); err == nil {
			var raw map[string]interface{}
			if err := yaml.Unmarshal(data, &raw); err == nil {
				for _, key := range []string{"x-oops-stack", "x-oops-stacks", "x-oops-services", "x-oops-depends"} {
					if v, ok := raw[key]; ok {
						extractServiceNames(v, depSet)
					}
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

	// 5. Remote capsule service templates (/@<remote>/services/*.yaml) for backward compatibility
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
		Warnings: warnings,
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
			inspector.CapsuleStackFiles = append(inspector.CapsuleStackFiles, "compose."+sName+".yaml")
		} else {
			inspector.StackFiles = append(inspector.StackFiles, "compose."+sName+".yaml")
		}
	}

	// 7. Locate config/<service> directories (and optional legacy services/<service>.yaml if present)
	templateMap := mapWorkspaceServiceTemplates(filepath.Join(workDir, "services"))
	seenServiceFiles := make(map[string]bool)
	seenConfigDirs := make(map[string]bool)

	// Collect all service names from required stacks + depSet to check config/<service>
	configCandidates := make(map[string]struct{})
	for dep := range depSet {
		configCandidates[dep] = struct{}{}
	}
	for s := range definedServices {
		configCandidates[s] = struct{}{}
	}

	for dep := range configCandidates {
		cleanDep := dep
		if strings.HasPrefix(cleanDep, "/") {
			parts := strings.Split(strings.TrimPrefix(cleanDep, "/"), "/")
			if len(parts) == 2 {
				cleanDep = parts[1]
			} else {
				continue
			}
		}

		// Legacy service template support if workspace still has services/
		if _, isDefined := definedServices[cleanDep]; !isDefined {
			templateBase := cleanDep
			if mapped, ok := templateMap[cleanDep]; ok {
				templateBase = mapped
			}
			if _, isTmplDefined := definedServices[templateBase]; !isTmplDefined {
				svcFile := filepath.Join("services", templateBase+".yaml")
				if !seenServiceFiles[svcFile] {
					if _, err := os.Stat(filepath.Join(workDir, svcFile)); err == nil {
						seenServiceFiles[svcFile] = true
						inspector.ServiceFiles = append(inspector.ServiceFiles, svcFile)
					}
				}
			}
		}

		configDir := filepath.Join("config", cleanDep)
		if !seenConfigDirs[configDir] {
			// Skip root config/<service> if capsule already overrides config/<service>
			if fi, err := os.Stat(filepath.Join(capsule, configDir)); err == nil && fi.IsDir() {
				continue
			}
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
