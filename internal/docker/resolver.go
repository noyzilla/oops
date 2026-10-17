package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolvedTarget represents a target service discovered in a compose stack
type ResolvedTarget struct {
	StackName     string            `json:"stack_name"`
	ComposePath   string            `json:"compose_path"`
	ComposeFiles  []string          `json:"compose_files,omitempty"`
	ServiceName   string            `json:"service_name"`
	ContainerName string            `json:"container_name"`
	ContainerID   string            `json:"container_id"`
	Image         string            `json:"image"`
	Labels        map[string]string `json:"labels"`
}

// MatchWildcard tests if a candidate string matches a target pattern
func MatchWildcard(pattern, candidate string) bool {
	if pattern == candidate {
		return true
	}

	// Double Dot (..) Wildcard Protocol
	if strings.HasPrefix(pattern, "..") && strings.HasSuffix(pattern, "..") && len(pattern) > 4 {
		substr := pattern[2 : len(pattern)-2]
		return strings.Contains(candidate, substr)
	}

	if strings.HasSuffix(pattern, "..") {
		prefix := strings.TrimSuffix(pattern, "..")
		return strings.HasPrefix(candidate, prefix)
	}

	if strings.HasPrefix(pattern, "..") {
		suffix := strings.TrimPrefix(pattern, "..")
		return strings.HasSuffix(candidate, suffix)
	}

	// Fallback to filepath.Match for standard globs if quotes were used
	if matched, err := filepath.Match(pattern, candidate); err == nil && matched {
		return true
	}

	return false
}

// DiscoverStacks finds all compose stacks in workDir strictly adhering to .yaml extension
func DiscoverStacks(workDir string) ([]string, map[string]string, error) {
	composeMap := make(map[string]string)
	discovered := make(map[string]bool)

	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read directory %s: %w", workDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Strict .yaml extension: ignore any .yml or non-yaml files
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}

		// Ignore untracked local override files and example files
		if strings.HasSuffix(name, ".local.yaml") || strings.HasSuffix(name, ".example") {
			continue
		}

		// Match root compose.yaml for "." stack
		if name == "compose.yaml" {
			composeMap["."] = filepath.Join(workDir, name)
			discovered["."] = true
			continue
		}

		// Match compose.<stack>.yaml
		if strings.HasPrefix(name, "compose.") {
			stack := strings.TrimSuffix(strings.TrimPrefix(name, "compose."), ".yaml")
			if stack != "" {
				composeMap[stack] = filepath.Join(workDir, name)
				discovered[stack] = true
			}
		}
	}

	var orderedStacks []string
	for s := range discovered {
		orderedStacks = append(orderedStacks, s)
	}
	sort.Strings(orderedStacks)

	return orderedStacks, composeMap, nil
}

// DiscoverLayers is an alias to DiscoverStacks for backward compatibility
func DiscoverLayers(workDir string) ([]string, map[string]string, error) {
	return DiscoverStacks(workDir)
}

// BuildComposeChain constructs the multi -f chain with strict tail local overrides
func BuildComposeChain(workDir string, baseFiles []string, project string) []string {
	var chain []string
	seen := make(map[string]bool)

	// 1. Root compose.yaml anchor (if exists)
	rootCompose := filepath.Join(workDir, "compose.yaml")
	if _, err := os.Stat(rootCompose); err == nil {
		chain = append(chain, rootCompose)
		seen[rootCompose] = true
	}

	// 2. Base files (stack manifests)
	for _, f := range baseFiles {
		if f != "" && !seen[f] {
			chain = append(chain, f)
			seen[f] = true
		}
	}

	// 3. Stack-specific local override: compose.<stack>.local.yaml
	if project != "" && project != "." {
		projLocal := filepath.Join(workDir, fmt.Sprintf("compose.%s.local.yaml", project))
		if _, err := os.Stat(projLocal); err == nil && !seen[projLocal] {
			chain = append(chain, projLocal)
			seen[projLocal] = true
		}
	}

	// 4. Global machine local override: compose.local.yaml (tailmost)
	globalLocal := filepath.Join(workDir, "compose.local.yaml")
	if _, err := os.Stat(globalLocal); err == nil && !seen[globalLocal] {
		chain = append(chain, globalLocal)
		seen[globalLocal] = true
	}

	return chain
}

// ResolveTargets resolves target patterns to matching services in the workDir preserving x-oops-stack order.
func ResolveTargets(workDir string, targets []string) ([]ResolvedTarget, error) {
	orderedStacks, composeMap, err := DiscoverStacks(workDir)
	if err != nil {
		return nil, err
	}

	type stackData struct {
		stackName   string
		composePath string
		services    map[string]ComposeService
		xStack      []string
		xServices   []string
		xDepends    []string
	}

	stacks := make(map[string]*stackData)

	for _, s := range orderedStacks {
		cPath := composeMap[s]
		cfg, err := ParseComposeFile(cPath)
		if err != nil {
			return nil, err
		}
		stacks[s] = &stackData{
			stackName:   s,
			composePath: cPath,
			services:    cfg.Services,
			xStack:      cfg.GetXOopsStack(),
			xServices:   cfg.XOopsServices,
			xDepends:    cfg.XOopsDepends,
		}
	}

	oopsCfg, _ := LoadOopsConfig(workDir)
	if oopsCfg == nil {
		oopsCfg = &OopsConfig{Registries: make(map[string]string)}
	}

	// If no targets supplied: target "." stack (compose.yaml)
	if len(targets) == 0 {
		targets = []string{"/."}
	}

	// Ordered list of resolved service paths ("/<stack>/<service>")
	var orderedServicePaths []string
	inTargetSet := make(map[string]bool)
	inProgress := make(map[string]bool)

	addServicePath := func(srvPath string) {
		if !inTargetSet[srvPath] {
			inTargetSet[srvPath] = true
			orderedServicePaths = append(orderedServicePaths, srvPath)
		}
	}

	// resolveCrossStackService searches for a service name across stacks (preferring db, edge, storage, tools, then alphabetical)
	resolveCrossStackService := func(srvName string) (string, string) {
		preferredOrder := []string{"db", "edge", "storage", "tools"}
		for _, pStack := range preferredOrder {
			if sd, ok := stacks[pStack]; ok {
				if _, hasSrv := sd.services[srvName]; hasSrv {
					return pStack, srvName
				}
			}
		}
		for _, sName := range orderedStacks {
			if sd, ok := stacks[sName]; ok {
				if _, hasSrv := sd.services[srvName]; hasSrv {
					return sName, srvName
				}
			}
		}
		return "", ""
	}

	// expandTargetItem expands a target token or x-oops-stack entry into orderedServicePaths
	var expandTargetItem func(item string, currentStack string) error
	expandTargetItem = func(item string, currentStack string) error {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil
		}

		// Case 1: Leading slash "/<stack>" or "/<stack>/<service>"
		if strings.HasPrefix(item, "/") {
			trimmed := strings.TrimPrefix(item, "/")
			parts := strings.SplitN(trimmed, "/", 2)
			targetStack := parts[0]

			sd, ok := stacks[targetStack]
			if !ok {
				return fmt.Errorf("stack not found: /%s", targetStack)
			}

			// Specific service inside stack: "/<stack>/<service>"
			if len(parts) == 2 {
				srvName := parts[1]
				if _, hasSrv := sd.services[srvName]; !hasSrv {
					return fmt.Errorf("service %q not found in stack /%s", srvName, targetStack)
				}
				addServicePath("/" + targetStack + "/" + srvName)
				return nil
			}

			// Whole stack: "/<stack>"
			stackKey := "stack:/" + targetStack
			if inProgress[stackKey] {
				return fmt.Errorf("circular x-oops-stack reference involving /%s", targetStack)
			}
			inProgress[stackKey] = true
			defer func() { inProgress[stackKey] = false }()

			// If the stack defines x-oops-stack, expand strictly those items in declared order!
			if len(sd.xStack) > 0 {
				for _, subItem := range sd.xStack {
					if err := expandTargetItem(subItem, targetStack); err != nil {
						return err
					}
				}
				return nil
			}

			// Otherwise, include all services defined in this stack (sorted by name for determinism)
			var srvNames []string
			for srvName := range sd.services {
				srvNames = append(srvNames, srvName)
			}
			sort.Strings(srvNames)
			for _, srvName := range srvNames {
				addServicePath("/" + targetStack + "/" + srvName)
			}
			return nil
		}

		// Case 2: Bare item inside a stack's x-oops-stack (e.g. "mysql" inside compose.db.yaml)
		if currentStack != "" {
			if sd, ok := stacks[currentStack]; ok {
				if _, hasLocal := sd.services[item]; hasLocal {
					addServicePath("/" + currentStack + "/" + item)
					return nil
				}
			}
			// Fallback: if bare name matches another stack name (e.g. "edge" or "db")
			if _, isStack := stacks[item]; isStack {
				return expandTargetItem("/"+item, "")
			}
		}

		// Case 3: CLI bare service name, wildcard, or image query
		found := false
		isImageQuery := strings.HasPrefix(item, "img:") || strings.HasPrefix(item, "image:")
		if !isImageQuery {
			for _, sName := range orderedStacks {
				sd := stacks[sName]
				if _, ok := sd.services[item]; ok {
					addServicePath("/" + sName + "/" + item)
					found = true
				}
				if MatchWildcard(item, "dummy") || strings.Contains(item, "..") {
					var srvNames []string
					for srvName := range sd.services {
						srvNames = append(srvNames, srvName)
					}
					sort.Strings(srvNames)
					for _, srvName := range srvNames {
						if MatchWildcard(item, srvName) {
							addServicePath("/" + sName + "/" + srvName)
							found = true
						}
					}
				}
			}
			// Also allow bare stack name on CLI if no service matched
			if !found {
				if _, isStack := stacks[item]; isStack {
					return expandTargetItem("/"+item, "")
				}
			}
		}

		if !found {
			expandedImage := ExpandImageAlias(item, oopsCfg.Registries)
			cleanImageQuery := strings.TrimPrefix(strings.TrimPrefix(item, "image:"), "img:")

			for _, sName := range orderedStacks {
				sd := stacks[sName]
				var srvNames []string
				for srvName := range sd.services {
					srvNames = append(srvNames, srvName)
				}
				sort.Strings(srvNames)
				for _, srvName := range srvNames {
					s := sd.services[srvName]
					isImageMatch := s.Image == expandedImage ||
						s.Image == cleanImageQuery ||
						strings.HasSuffix(s.Image, "/"+cleanImageQuery) ||
						strings.HasSuffix(s.Image, ":"+cleanImageQuery) ||
						MatchWildcard(item, s.Image)

					if isImageMatch {
						addServicePath("/" + sName + "/" + srvName)
						found = true
					}
				}
			}
		}

		if !found {
			return fmt.Errorf("target service not found: %s", item)
		}
		return nil
	}

	for _, t := range targets {
		if err := expandTargetItem(t, ""); err != nil {
			return nil, err
		}
	}

	// resolveDep resolves a dependency reference ("/<stack>/<service>", "/<stack>", or bare service name)
	resolveDep := func(dep string, fromStack string) (string, string) {
		if strings.HasPrefix(dep, "/") {
			trimmed := strings.TrimPrefix(dep, "/")
			parts := strings.SplitN(trimmed, "/", 2)
			if len(parts) == 2 {
				return parts[0], parts[1]
			}
			return parts[0], ""
		}
		if fromStack != "" {
			if sd, ok := stacks[fromStack]; ok {
				if _, isLocal := sd.services[dep]; isLocal {
					return fromStack, dep
				}
			}
		}
		return resolveCrossStackService(dep)
	}

	// Build final ResolvedTarget list respecting dependency order while preserving x-oops-stack sequence
	var finalPaths []string
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	var visit func(srvPath string) error
	visit = func(srvPath string) error {
		if visiting[srvPath] {
			return fmt.Errorf("circular dependency detected involving %s", srvPath)
		}
		if visited[srvPath] {
			return nil
		}
		visiting[srvPath] = true

		trimmed := strings.TrimPrefix(srvPath, "/")
		lastSlash := strings.LastIndex(trimmed, "/")
		if lastSlash == -1 {
			return fmt.Errorf("invalid service path: %s", srvPath)
		}
		stackName, srvName := trimmed[:lastSlash], trimmed[lastSlash+1:]
		sd, ok := stacks[stackName]
		if !ok {
			return fmt.Errorf("stack not found: %s", stackName)
		}
		srv, hasSrv := sd.services[srvName]
		if !hasSrv {
			return fmt.Errorf("service %s not found in stack %s", srvName, stackName)
		}

		// Visit stack-level x-oops-depends if present
		for _, dep := range sd.xDepends {
			depStack, depSrv := resolveDep(dep, stackName)
			if depStack != "" {
				if depSrv != "" {
					depPath := "/" + depStack + "/" + depSrv
					if depPath != srvPath {
						if err := visit(depPath); err != nil {
							return err
						}
					}
				} else if depSd, ok := stacks[depStack]; ok {
					var depSrvNames []string
					if len(depSd.xStack) > 0 {
						for _, item := range depSd.xStack {
							if _, has := depSd.services[item]; has {
								depSrvNames = append(depSrvNames, item)
							}
						}
					} else {
						for s := range depSd.services {
							depSrvNames = append(depSrvNames, s)
						}
						sort.Strings(depSrvNames)
					}
					for _, s := range depSrvNames {
						depPath := "/" + depStack + "/" + s
						if depPath != srvPath {
							if err := visit(depPath); err != nil {
								return err
							}
						}
					}
				}
			}
		}

		// Visit service-level depends_on
		for _, dep := range srv.ParsedDependsOn {
			depStack, depSrv := resolveDep(dep, stackName)
			if depStack != "" && depSrv != "" {
				depPath := "/" + depStack + "/" + depSrv
				if depPath != srvPath {
					if err := visit(depPath); err != nil {
						return err
					}
				}
			}
		}

		visiting[srvPath] = false
		visited[srvPath] = true
		finalPaths = append(finalPaths, srvPath)
		return nil
	}

	for _, srvPath := range orderedServicePaths {
		if err := visit(srvPath); err != nil {
			return nil, err
		}
	}

	var results []ResolvedTarget
	for _, srvPath := range finalPaths {
		trimmed := strings.TrimPrefix(srvPath, "/")
		lastSlash := strings.LastIndex(trimmed, "/")
		stackName, srvName := trimmed[:lastSlash], trimmed[lastSlash+1:]
		sd := stacks[stackName]
		srv := sd.services[srvName]

		var baseFiles []string
		projectName := stackName

		if stackName != "." {
			// Prepend cross-stack dependency compose files if needed so Docker Compose validates depends_on
			for _, dep := range srv.ParsedDependsOn {
				if _, isLocal := sd.services[dep]; isLocal {
					continue
				}
				depStack, _ := resolveDep(dep, stackName)
				if depStack != "" && depStack != stackName {
					if depSd, ok := stacks[depStack]; ok && depSd.composePath != "" {
						baseFiles = append(baseFiles, depSd.composePath)
					}
				}
			}
			for _, dep := range sd.xDepends {
				if _, isLocal := sd.services[dep]; isLocal {
					continue
				}
				depStack, _ := resolveDep(dep, stackName)
				if depStack != "" && depStack != stackName {
					if depSd, ok := stacks[depStack]; ok && depSd.composePath != "" {
						baseFiles = append(baseFiles, depSd.composePath)
					}
				}
			}
			for _, s := range sd.xServices {
				if _, isLocal := sd.services[s]; isLocal {
					continue
				}
				depStack, _ := resolveDep(s, stackName)
				if depStack != "" && depStack != stackName {
					if depSd, ok := stacks[depStack]; ok && depSd.composePath != "" {
						baseFiles = append(baseFiles, depSd.composePath)
					}
				}
			}
			baseFiles = append(baseFiles, sd.composePath)
		}

		composeChain := BuildComposeChain(workDir, baseFiles, projectName)
		primaryPath := sd.composePath
		if len(composeChain) > 0 {
			primaryPath = composeChain[0]
		}

		results = append(results, ResolvedTarget{
			StackName:     stackName,
			ComposePath:   primaryPath,
			ComposeFiles:  composeChain,
			ServiceName:   srvName,
			ContainerName: srv.ContainerName,
			Image:         srv.Image,
			Labels:        srv.ParsedLabels,
		})
	}

	return results, nil
}

// ResolveTargetsByImage resolves targets matching a container image
func ResolveTargetsByImage(workDir string, imageQuery string) ([]ResolvedTarget, error) {
	return ResolveTargets(workDir, []string{"img:" + imageQuery})
}

// ResolveTargetsWithExceptions resolves targets and excludes any services matched by exceptTargets
func ResolveTargetsWithExceptions(workDir string, targets []string, exceptTargets []string) ([]ResolvedTarget, error) {
	var baseTargets []ResolvedTarget
	var err error

	if len(targets) == 0 {
		baseTargets, err = ResolveTargets(workDir, nil)
		if err != nil {
			return nil, err
		}
	} else {
		baseTargets, err = ResolveTargets(workDir, targets)
		if err != nil {
			return nil, err
		}
	}

	if len(exceptTargets) == 0 {
		return baseTargets, nil
	}

	excluded, err := ResolveTargets(workDir, exceptTargets)
	if err != nil {
		return nil, err
	}

	excludeMap := make(map[string]bool)
	for _, ex := range excluded {
		excludeMap[ex.StackName+"/"+ex.ServiceName] = true
		if ex.ContainerName != "" {
			excludeMap[ex.ContainerName] = true
		}
	}

	var filtered []ResolvedTarget
	for _, t := range baseTargets {
		key := t.StackName + "/" + t.ServiceName
		if !excludeMap[key] && !excludeMap[t.ContainerName] {
			filtered = append(filtered, t)
		}
	}

	return filtered, nil
}
