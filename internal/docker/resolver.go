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
	ServiceName   string            `json:"service_name"`
	ContainerName string            `json:"container_name"`
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

// DiscoverStacks finds all stack directories in workDir (under stacks/ or directly) containing compose files
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

		// Match compose.yml or docker-compose.yml for root "." stack
		if name == "compose.yml" || name == "compose.yaml" || name == "docker-compose.yml" || name == "docker-compose.yaml" {
			composeMap["."] = filepath.Join(workDir, name)
			discovered["."] = true
			continue
		}

		// Match compose.<stack>.yml pattern
		var stack string
		if strings.HasPrefix(name, "compose.") {
			if strings.HasSuffix(name, ".yml") {
				stack = strings.TrimSuffix(strings.TrimPrefix(name, "compose."), ".yml")
			} else if strings.HasSuffix(name, ".yaml") {
				stack = strings.TrimSuffix(strings.TrimPrefix(name, "compose."), ".yaml")
			}
		} else if strings.HasPrefix(name, "docker-compose.") {
			if strings.HasSuffix(name, ".yml") {
				stack = strings.TrimSuffix(strings.TrimPrefix(name, "docker-compose."), ".yml")
			} else if strings.HasSuffix(name, ".yaml") {
				stack = strings.TrimSuffix(strings.TrimPrefix(name, "docker-compose."), ".yaml")
			}
		}

		if stack != "" {
			composeMap[stack] = filepath.Join(workDir, name)
			discovered[stack] = true
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

// ResolveTargets resolves target patterns to matching services in the workDir
func ResolveTargets(workDir string, targets []string) ([]ResolvedTarget, error) {
	orderedStacks, composeMap, err := DiscoverStacks(workDir)
	if err != nil {
		return nil, err
	}

	// Load all services across stacks
	type stackData struct {
		stackName   string
		composePath string
		services    map[string]ComposeService
		dependsOn   []string
	}

	stacks := make(map[string]*stackData)
	var allStacks []string

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
			dependsOn:   cfg.OopsDependsOn,
		}
		allStacks = append(allStacks, s)
	}

	oopsCfg, _ := LoadOopsConfig(workDir)
	if oopsCfg == nil {
		oopsCfg = &OopsConfig{Registries: make(map[string]string)}
	}

	// If no targets supplied: target . stack (compose.yml)
	if len(targets) == 0 {
		targets = []string{"/."}
	}


	expandedTargets := targets

	// Identify explicitly targeted stacks and services
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
				// Bare target (e.g. "mysql"). We must search all stacks to find this service.
				for sName, sd := range stacks {
					if _, ok := sd.services[target]; ok {
						targetStacksMap[sName] = true
						targetServicesMap["/"+sName+"/"+target] = true
						found = true
					}
					// Also handle wildcard matching
					if MatchWildcard(target, "dummy") || strings.Contains(target, "..") { // rough check for wildcard
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

			if !found {
				// Try as image query (or if it was explicitly prefixed)
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

	// DFS for topological sort of stacks
	var topoOrdered []string
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	var visit func(stackName string, callStack []string) error
	visit = func(stackName string, callStack []string) error {
		if visiting[stackName] {
			return fmt.Errorf("circular stack dependency detected: %s -> %s", strings.Join(callStack, " -> "), stackName)
		}
		if visited[stackName] {
			return nil
		}
		
		sd, exists := stacks[stackName]
		if !exists {
			return fmt.Errorf("stack not found: %s", stackName)
		}

		visiting[stackName] = true
		callStack = append(callStack, stackName)

		// Visit dependencies first
		for _, dep := range sd.dependsOn {
			depStack := strings.TrimPrefix(dep, "/")
			if err := visit(depStack, callStack); err != nil {
				return err
			}
		}

		visiting[stackName] = false
		visited[stackName] = true
		topoOrdered = append(topoOrdered, stackName)
		return nil
	}

	// Build topological order for all target stacks
	var tStacks []string
	for s := range targetStacksMap {
		tStacks = append(tStacks, s)
	}
	sort.Strings(tStacks)

	for _, s := range tStacks {
		if err := visit(s, nil); err != nil {
			return nil, err
		}
	}

	var results []ResolvedTarget
	seen := make(map[string]bool)

	for _, sName := range topoOrdered {
		sd := stacks[sName]
		
		hasSpecificServiceTargets := false
		for tgt := range targetServicesMap {
			if strings.HasPrefix(tgt, "/"+sName+"/") {
				hasSpecificServiceTargets = true
				break
			}
		}

		isFullStackTarget := !hasSpecificServiceTargets || !targetStacksMap[sName]

		var sNames []string
		for srvName := range sd.services {
			sNames = append(sNames, srvName)
		}
		sort.Strings(sNames)

		for _, srvName := range sNames {
			if !isFullStackTarget {
				if !targetServicesMap["/"+sName+"/"+srvName] {
					continue
				}
			}

			key := sName + "/" + srvName
			if !seen[key] {
				seen[key] = true
				s := sd.services[srvName]
				results = append(results, ResolvedTarget{
					StackName:     sName,
					ComposePath:   sd.composePath,
					ServiceName:   srvName,
					ContainerName: s.ContainerName,
					Image:         s.Image,
					Labels:        s.ParsedLabels,
				})
			}
		}
	}

	return results, nil
}

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
