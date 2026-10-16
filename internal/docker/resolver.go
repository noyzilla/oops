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

		// Ignore untracked local override files
		if strings.HasSuffix(name, ".local.yaml") {
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

	// Fallback to oops.yaml for "." stack if compose.yaml is not present
	if !discovered["."] {
		oopsFile := filepath.Join(workDir, "oops.yaml")
		if _, err := os.Stat(oopsFile); err == nil {
			composeMap["."] = oopsFile
			discovered["."] = true
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

// DiscoverServiceTemplates finds all atomic templates in workDir/services/<service>.yaml
func DiscoverServiceTemplates(workDir string) map[string]string {
	templates := make(map[string]string)
	servicesDir := filepath.Join(workDir, "services")
	entries, err := os.ReadDir(servicesDir)
	if err != nil {
		return templates
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".local.yaml") {
			svc := strings.TrimSuffix(name, ".yaml")
			templates[svc] = filepath.Join(servicesDir, name)
		}
	}
	return templates
}

// DiscoverGroups reads x-oops-groups from root compose.yaml (and oops.yaml fallback)
func DiscoverGroups(workDir string) map[string][]string {
	groups := make(map[string][]string)
	for _, fname := range []string{"compose.yaml", "oops.yaml"} {
		composePath := filepath.Join(workDir, fname)
		cfg, err := ParseComposeFile(composePath)
		if err == nil && cfg != nil && cfg.XOopsGroups != nil {
			for k, v := range cfg.XOopsGroups {
				groups[k] = v
			}
		}
	}
	return groups
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

	// 2. Orchestrator oops.yaml anchor (if exists)
	oopsCompose := filepath.Join(workDir, "oops.yaml")
	if _, err := os.Stat(oopsCompose); err == nil && !seen[oopsCompose] {
		chain = append(chain, oopsCompose)
		seen[oopsCompose] = true
	}

	// 3. Base files (templates, projects)
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

// ResolveTargets resolves target patterns to matching services in the workDir
func ResolveTargets(workDir string, targets []string) ([]ResolvedTarget, error) {
	orderedStacks, composeMap, err := DiscoverStacks(workDir)
	if err != nil {
		return nil, err
	}

	templates := DiscoverServiceTemplates(workDir)
	groups := DiscoverGroups(workDir)

	type stackData struct {
		stackName   string
		composePath string
		services    map[string]ComposeService
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
			xServices:   cfg.XOopsServices,
			xDepends:    cfg.XOopsDepends,
		}
	}

	// Also index standalone templates from services/ as atomic stacks so they can be discovered
	for tmplName, tmplPath := range templates {
		cfg, err := ParseComposeFile(tmplPath)
		if err == nil && cfg != nil {
			// If service template has no explicitly named service or single service, index it
			stacks["services/"+tmplName] = &stackData{
				stackName:   "services/" + tmplName,
				composePath: tmplPath,
				services:    cfg.Services,
			}
		}
	}

	// Index services from oops.yaml into root "." stack so orchestrator daemon service is discoverable
	oopsPath := filepath.Join(workDir, "oops.yaml")
	if _, err := os.Stat(oopsPath); err == nil {
		if oopsCfg, err := ParseComposeFile(oopsPath); err == nil && oopsCfg != nil && len(oopsCfg.Services) > 0 {
			if dotStack, ok := stacks["."]; ok {
				if dotStack.services == nil {
					dotStack.services = make(map[string]ComposeService)
				}
				for k, v := range oopsCfg.Services {
					dotStack.services[k] = v
				}
			} else {
				stacks["."] = &stackData{
					stackName:   ".",
					composePath: oopsPath,
					services:    oopsCfg.Services,
				}
			}
		}
	}

	oopsCfg, _ := LoadOopsConfig(workDir)
	if oopsCfg == nil {
		oopsCfg = &OopsConfig{Registries: make(map[string]string)}
	}

	// If no targets supplied: target . stack (compose.yaml)
	if len(targets) == 0 {
		targets = []string{"/."}
	}

	// Expand group targets (+<group>)
	var expandedTargets []string
	for _, target := range targets {
		if strings.HasPrefix(target, "+") {
			groupName := strings.TrimPrefix(target, "+")
			memberServices, ok := groups[groupName]
			if !ok || len(memberServices) == 0 {
				return nil, fmt.Errorf("service group +%s not found or empty in compose.yaml", groupName)
			}
			for _, m := range memberServices {
				expandedTargets = append(expandedTargets, m)
			}
		} else {
			expandedTargets = append(expandedTargets, target)
		}
	}

	targetStacksMap := make(map[string]bool)
	targetServicesMap := make(map[string]bool)

	for _, target := range expandedTargets {
		if strings.HasPrefix(target, "/") {
			trimmed := strings.TrimPrefix(target, "/")
			parts := strings.SplitN(trimmed, "/", 2)
			targetStack := parts[0]
			targetStacksMap[targetStack] = true
			if len(parts) == 2 {
				targetServicesMap[target] = true
			}
		} else {
			found := false
			isImageQuery := strings.HasPrefix(target, "img:") || strings.HasPrefix(target, "image:")
			if !isImageQuery {
				// 1. First, check if exact service template exists in services/<target>.yaml
				if _, ok := templates[target]; ok {
					sName := "services/" + target
					targetStacksMap[sName] = true
					// Match all services inside this template, or the service with same name
					if sd, hasSd := stacks[sName]; hasSd {
						for srvName := range sd.services {
							targetServicesMap["/"+sName+"/"+srvName] = true
							found = true
						}
					}
					if !found {
						targetServicesMap["/"+sName+"/"+target] = true
						found = true
					}
				}

				// 2. Search across loaded stacks for this service name
				if !found {
					for sName, sd := range stacks {
						if _, ok := sd.services[target]; ok {
							targetStacksMap[sName] = true
							targetServicesMap["/"+sName+"/"+target] = true
							found = true
						}
						// Wildcard matching
						if MatchWildcard(target, "dummy") || strings.Contains(target, "..") {
							for srvName := range sd.services {
								if MatchWildcard(target, srvName) {
									targetStacksMap[sName] = true
									targetServicesMap["/"+sName+"/"+srvName] = true
									found = true
								}
							}
						}
					}
				}
			}

			if !found {
				// Try as image query
				expandedImage := ExpandImageAlias(target, oopsCfg.Registries)
				cleanImageQuery := strings.TrimPrefix(strings.TrimPrefix(target, "image:"), "img:")

				for sName, sd := range stacks {
					for srvName, s := range sd.services {
						isImageMatch := s.Image == expandedImage ||
							s.Image == cleanImageQuery ||
							strings.HasSuffix(s.Image, "/"+cleanImageQuery) ||
							strings.HasSuffix(s.Image, ":"+cleanImageQuery) ||
							MatchWildcard(target, s.Image)

						if isImageMatch {
							targetStacksMap[sName] = true
							targetServicesMap["/"+sName+"/"+srvName] = true
							found = true
						}
					}
				}
			}

			if !found {
				return nil, fmt.Errorf("target service not found: %s", target)
			}
		}
	}

	// Map service names to their stack for cross-stack dependency resolution
	serviceToStack := make(map[string]string)
	for sName, sd := range stacks {
		for srvName := range sd.services {
			serviceToStack[srvName] = sName
		}
	}

	// Convert any full stack targets into individual services
	for sName := range targetStacksMap {
		hasSpecificServiceTargets := false
		for tgt := range targetServicesMap {
			if strings.HasPrefix(tgt, "/"+sName+"/") {
				hasSpecificServiceTargets = true
				break
			}
		}
		if !hasSpecificServiceTargets {
			if sd, ok := stacks[sName]; ok {
				for srvName := range sd.services {
					targetServicesMap["/"+sName+"/"+srvName] = true
				}
			}
		}
	}

	// resolveDep resolves a dependency reference (starts with / for stack/service, or template name in services/)
	resolveDep := func(dep string) (string, string) {
		if strings.HasPrefix(dep, "/") {
			trimmed := strings.TrimPrefix(dep, "/")
			parts := strings.SplitN(trimmed, "/", 2)
			if len(parts) == 2 {
				return parts[0], parts[1]
			}
			return parts[0], ""
		}
		// No slash: refers to services/ template
		if _, ok := templates[dep]; ok {
			sName := "services/" + dep
			if tSd, hasTSd := stacks[sName]; hasTSd {
				for srvName := range tSd.services {
					return sName, srvName
				}
			}
			return sName, dep
		}
		// Search if dep is a service name inside any service template
		for tmplName := range templates {
			sName := "services/" + tmplName
			if tSd, hasTSd := stacks[sName]; hasTSd {
				if _, hasSrv := tSd.services[dep]; hasSrv {
					return sName, dep
				}
			}
		}
		return "", ""
	}

	// Recursively resolve native depends_on dependencies
	var expandDeps func(srvPath string)
	expandDeps = func(srvPath string) {
		trimmed := strings.TrimPrefix(srvPath, "/")
		lastSlash := strings.LastIndex(trimmed, "/")
		if lastSlash == -1 {
			return
		}
		stackName, srvName := trimmed[:lastSlash], trimmed[lastSlash+1:]
		sd, ok := stacks[stackName]
		if !ok {
			return
		}
		srv, ok := sd.services[srvName]
		if !ok {
			return
		}

		for _, dep := range srv.ParsedDependsOn {
			// Local Definition Takes Precedence (Mental Model 1):
			// If dependency is defined locally within this stack, use local service!
			if _, isLocal := sd.services[dep]; isLocal {
				depPath := "/" + stackName + "/" + dep
				if !targetServicesMap[depPath] {
					targetServicesMap[depPath] = true
					expandDeps(depPath)
				}
				continue
			}

			depStack, depSrv := resolveDep(dep)
			if depStack != "" && depSrv != "" {
				depPath := "/" + depStack + "/" + depSrv
				if !targetServicesMap[depPath] {
					targetServicesMap[depPath] = true
					targetStacksMap[depStack] = true
					expandDeps(depPath)
				}
			}
		}
	}

	var initialTargets []string
	for tgt := range targetServicesMap {
		initialTargets = append(initialTargets, tgt)
	}
	for _, tgt := range initialTargets {
		expandDeps(tgt)
	}

	// Also expand stack-level x-oops-depends for targeted stacks
	for sName := range targetStacksMap {
		if sd, ok := stacks[sName]; ok {
			for _, dep := range sd.xDepends {
				if _, isLocal := sd.services[dep]; isLocal {
					depPath := "/" + sName + "/" + dep
					if !targetServicesMap[depPath] {
						targetServicesMap[depPath] = true
						expandDeps(depPath)
					}
					continue
				}
				depStack, depSrv := resolveDep(dep)
				if depStack == "" {
					continue
				}
				targetStacksMap[depStack] = true
				if depSrv != "" {
					depPath := "/" + depStack + "/" + depSrv
					if !targetServicesMap[depPath] {
						targetServicesMap[depPath] = true
						expandDeps(depPath)
					}
				} else if depSd, ok := stacks[depStack]; ok {
					for srvName := range depSd.services {
						depPath := "/" + depStack + "/" + srvName
						if !targetServicesMap[depPath] {
							targetServicesMap[depPath] = true
							expandDeps(depPath)
						}
					}
				}
			}
		}
	}

	// Topological sort
	var sortedResults []ResolvedTarget
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
		srv := sd.services[srvName]

		// Visit stack-level x-oops-depends first
		for _, dep := range sd.xDepends {
			if _, isLocal := sd.services[dep]; isLocal {
				depPath := "/" + stackName + "/" + dep
				if depPath != srvPath && targetServicesMap[depPath] {
					if err := visit(depPath); err != nil {
						return err
					}
				}
				continue
			}
			depStack, depSrv := resolveDep(dep)
			if depStack != "" {
				if depSrv != "" {
					depPath := "/" + depStack + "/" + depSrv
					if depPath != srvPath && targetServicesMap[depPath] {
						if err := visit(depPath); err != nil {
							return err
						}
					}
				} else if depSd, ok := stacks[depStack]; ok {
					for srvName := range depSd.services {
						depPath := "/" + depStack + "/" + srvName
						if depPath != srvPath && targetServicesMap[depPath] {
							if err := visit(depPath); err != nil {
								return err
							}
						}
					}
				}
			}
		}

		// Visit service depends_on dependencies
		for _, dep := range srv.ParsedDependsOn {
			if _, isLocal := sd.services[dep]; isLocal {
				depPath := "/" + stackName + "/" + dep
				if depPath != srvPath && targetServicesMap[depPath] {
					if err := visit(depPath); err != nil {
						return err
					}
				}
				continue
			}

			depStack, depSrv := resolveDep(dep)
			if depStack != "" && depSrv != "" {
				depPath := "/" + depStack + "/" + depSrv
				if targetServicesMap[depPath] {
					if err := visit(depPath); err != nil {
						return err
					}
				}
			}
		}

		visiting[srvPath] = false
		visited[srvPath] = true

		// Build multi -f compose files chain
		var baseFiles []string
		projectName := stackName

		if strings.HasPrefix(stackName, "services/") {
			projectName = ""
			baseFiles = append(baseFiles, sd.composePath)
		} else if stackName != "." {
			// Stack: prepend any service template dependencies
			for _, dep := range srv.ParsedDependsOn {
				// Local definition takes precedence: never pull in template if defined in stack!
				if _, isLocal := sd.services[dep]; isLocal {
					continue
				}
				depStack, _ := resolveDep(dep)
				if depStack != "" {
					if depSd, ok := stacks[depStack]; ok && depSd.composePath != "" {
						baseFiles = append(baseFiles, depSd.composePath)
					}
				}
			}
			for _, dep := range sd.xDepends {
				if _, isLocal := sd.services[dep]; isLocal {
					continue
				}
				depStack, _ := resolveDep(dep)
				if depStack != "" {
					if depSd, ok := stacks[depStack]; ok && depSd.composePath != "" {
						baseFiles = append(baseFiles, depSd.composePath)
					}
				}
			}
			for _, s := range sd.xServices {
				if _, isLocal := sd.services[s]; isLocal {
					continue
				}
				depStack, _ := resolveDep(s)
				if depStack != "" {
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

		sortedResults = append(sortedResults, ResolvedTarget{
			StackName:     stackName,
			ComposePath:   primaryPath,
			ComposeFiles:  composeChain,
			ServiceName:   srvName,
			ContainerName: srv.ContainerName,
			Image:         srv.Image,
			Labels:        srv.ParsedLabels,
		})
		return nil
	}

	var targetPaths []string
	for tgt := range targetServicesMap {
		targetPaths = append(targetPaths, tgt)
	}
	sort.Strings(targetPaths)

	for _, tgt := range targetPaths {
		if err := visit(tgt); err != nil {
			return nil, err
		}
	}

	return sortedResults, nil
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
