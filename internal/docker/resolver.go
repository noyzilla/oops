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

// Standard stack startup dependency order
var defaultStackOrder = []string{"edge", "db", "tool", "apps", "utils"}

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

	rootCandidates := []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}

	// 1. Primary Discovery: Check stacks/ subdirectory
	stacksDir := filepath.Join(workDir, "stacks")
	if fi, err := os.Stat(stacksDir); err == nil && fi.IsDir() {
		entries, err := os.ReadDir(stacksDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
					continue
				}
				name := entry.Name()
				stackSubDir := filepath.Join(stacksDir, name)
				for _, c := range rootCandidates {
					composePath := filepath.Join(stackSubDir, c)
					if _, err := os.Stat(composePath); err == nil {
						composeMap[name] = composePath
						discovered[name] = true
						break
					}
				}
			}
		}
	}

	// 2. Secondary Discovery: If no stacks found in stacks/, check top-level directories
	if len(discovered) == 0 {
		entries, err := os.ReadDir(workDir)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read directory %s: %w", workDir, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasPrefix(name, ".") || name == "data" || name == "backups" || name == "docs" || name == "scripts" {
				continue
			}

			stackDir := filepath.Join(workDir, name)
			for _, c := range rootCandidates {
				composePath := filepath.Join(stackDir, c)
				if _, err := os.Stat(composePath); err == nil {
					composeMap[name] = composePath
					discovered[name] = true
					break
				}
			}
		}
	}

	// 3. Fallback: Check root-level compose
	for _, c := range rootCandidates {
		p := filepath.Join(workDir, c)
		if _, err := os.Stat(p); err == nil {
			composeMap["."] = p
			discovered["."] = true
			break
		}
	}

	// Order stacks according to canonical dependency order, then custom stacks alphabetically
	var orderedStacks []string
	for _, s := range defaultStackOrder {
		if discovered[s] {
			orderedStacks = append(orderedStacks, s)
		}
	}

	var customStacks []string
	for s := range discovered {
		isDefault := false
		for _, ds := range defaultStackOrder {
			if s == ds {
				isDefault = true
				break
			}
		}
		if !isDefault && s != "." {
			customStacks = append(customStacks, s)
		}
	}
	sort.Strings(customStacks)
	orderedStacks = append(orderedStacks, customStacks...)

	if discovered["."] {
		orderedStacks = append(orderedStacks, ".")
	}

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
	type stackServices struct {
		stackName   string
		composePath string
		services    map[string]ComposeService
	}

	var allStacks []stackServices
	for _, s := range orderedStacks {
		cPath := composeMap[s]
		cfg, err := ParseComposeFile(cPath)
		if err != nil {
			return nil, err
		}
		allStacks = append(allStacks, stackServices{
			stackName:   s,
			composePath: cPath,
			services:    cfg.Services,
		})
	}

	// Load oops.yml configuration if available
	oopsCfg, _ := LoadOopsConfig(workDir)
	if oopsCfg == nil {
		oopsCfg = &OopsConfig{
			Registries: make(map[string]string),
			Profiles:   make(map[string][]string),
		}
	}

	// If no targets supplied: check if "default" profile is defined in oops.yml
	if len(targets) == 0 {
		if _, ok := oopsCfg.Profiles["default"]; ok {
			targets = []string{"@default"}
		}
	}

	// If still no targets: return all services across all stacks in dependency order
	if len(targets) == 0 {
		var results []ResolvedTarget
		for _, ss := range allStacks {
			// Sort service names within stack for deterministic order
			var sNames []string
			for sName := range ss.services {
				sNames = append(sNames, sName)
			}
			sort.Strings(sNames)

			for _, sName := range sNames {
				s := ss.services[sName]
				results = append(results, ResolvedTarget{
					StackName:     ss.stackName,
					ComposePath:   ss.composePath,
					ServiceName:   sName,
					ContainerName: s.ContainerName,
					Image:         s.Image,
					Labels:        s.ParsedLabels,
				})
			}
		}
		return results, nil
	}

	// Expand any @profile targets into concrete selectors
	var expandedTargets []string

	var expandTarget func(tgt string, callStack []string) error
	expandTarget = func(tgt string, callStack []string) error {
		if strings.HasPrefix(tgt, "@") {
			profileName := strings.TrimPrefix(tgt, "@")

			// Dynamic @all resolution: expand to all discovered stacks if not explicitly defined in oops.yml
			if profileName == "all" {
				if profileItems, exists := oopsCfg.Profiles["all"]; exists && len(profileItems) > 0 {
					for _, item := range profileItems {
						if err := expandTarget(item, append(callStack, profileName)); err != nil {
							return err
						}
					}
					return nil
				}
				// Built-in dynamic discovery across all stacks
				for _, ss := range allStacks {
					expandedTargets = append(expandedTargets, "/"+ss.stackName)
				}
				return nil
			}

			for _, s := range callStack {
				if s == profileName {
					return fmt.Errorf("circular profile dependency detected: %s", strings.Join(append(callStack, profileName), " -> "))
				}
			}

			profileItems, exists := oopsCfg.Profiles[profileName]
			if !exists {
				return fmt.Errorf("profile '%s' not found in oops.yml", profileName)
			}
			if len(profileItems) == 0 {
				return fmt.Errorf("profile '%s' is empty in oops.yml", profileName)
			}

			for _, item := range profileItems {
				if err := expandTarget(item, append(callStack, profileName)); err != nil {
					return err
				}
			}
			return nil
		}

		expandedTargets = append(expandedTargets, tgt)
		return nil
	}

	for _, t := range targets {
		if err := expandTarget(t, nil); err != nil {
			return nil, err
		}
	}

	var results []ResolvedTarget
	seen := make(map[string]bool)

	for _, target := range expandedTargets {
		matchedTarget := false

		// 1. Stack Target or Scoped Service (/stack or /stack/service)
		if strings.HasPrefix(target, "/") {
			trimmed := strings.TrimPrefix(target, "/")
			parts := strings.SplitN(trimmed, "/", 2)
			targetStack := parts[0]
			var targetService string
			if len(parts) == 2 {
				targetService = parts[1]
			}

			for _, ss := range allStacks {
				if ss.stackName == targetStack {
					var sNames []string
					for sName := range ss.services {
						sNames = append(sNames, sName)
					}
					sort.Strings(sNames)

					for _, sName := range sNames {
						if targetService != "" && sName != targetService {
							continue
						}
						s := ss.services[sName]
						key := ss.stackName + "/" + sName
						if !seen[key] {
							seen[key] = true
							results = append(results, ResolvedTarget{
								StackName:     ss.stackName,
								ComposePath:   ss.composePath,
								ServiceName:   sName,
								ContainerName: s.ContainerName,
								Image:         s.Image,
								Labels:        s.ParsedLabels,
							})
						}
						matchedTarget = true
					}
				}
			}
			if !matchedTarget {
				return nil, fmt.Errorf("target stack or service not found: %s", target)
			}
			continue
		}

		// 2. Service Name or Double Dot Wildcard Matching
		for _, ss := range allStacks {
			var sNames []string
			for sName := range ss.services {
				sNames = append(sNames, sName)
			}
			sort.Strings(sNames)

			for _, sName := range sNames {
				if MatchWildcard(target, sName) {
					s := ss.services[sName]
					key := ss.stackName + "/" + sName
					if !seen[key] {
						seen[key] = true
						results = append(results, ResolvedTarget{
							StackName:     ss.stackName,
							ComposePath:   ss.composePath,
							ServiceName:   sName,
							ContainerName: s.ContainerName,
							Image:         s.Image,
							Labels:        s.ParsedLabels,
						})
					}
					matchedTarget = true
				}
			}
		}

		// 3. Image / Registry Alias Matching (e.g. gar/app:v1.0, img:redis:7, or image substring)
		if !matchedTarget {
			expandedImage := ExpandImageAlias(target, oopsCfg.Registries)
			cleanImageQuery := strings.TrimPrefix(strings.TrimPrefix(target, "image:"), "img:")

			for _, ss := range allStacks {
				var sNames []string
				for sName := range ss.services {
					sNames = append(sNames, sName)
				}
				sort.Strings(sNames)

				for _, sName := range sNames {
					s := ss.services[sName]
					isImageMatch := s.Image == expandedImage ||
						s.Image == cleanImageQuery ||
						strings.HasSuffix(s.Image, "/"+cleanImageQuery) ||
						strings.HasSuffix(s.Image, ":"+cleanImageQuery) ||
						MatchWildcard(target, s.Image)

					if isImageMatch {
						key := ss.stackName + "/" + sName
						if !seen[key] {
							seen[key] = true
							results = append(results, ResolvedTarget{
								StackName:     ss.stackName,
								ComposePath:   ss.composePath,
								ServiceName:   sName,
								ContainerName: s.ContainerName,
								Image:         s.Image,
								Labels:        s.ParsedLabels,
							})
						}
						matchedTarget = true
					}
				}
			}
		}

		if !matchedTarget {
			return nil, fmt.Errorf("no matching services found for target: %s", target)
		}
	}

	return results, nil
}

// ResolveTargetsByImage resolves all services across all stacks matching the given image query or registry alias
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
