package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolvedTarget represents a target service discovered in a compose layer
type ResolvedTarget struct {
	LayerName     string            `json:"layer_name"`
	ComposePath   string            `json:"compose_path"`
	ServiceName   string            `json:"service_name"`
	ContainerName string            `json:"container_name"`
	Image         string            `json:"image"`
	Labels        map[string]string `json:"labels"`
}

// Standard layer startup dependency order
var defaultLayerOrder = []string{"edge", "db", "utils", "apps"}

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

// DiscoverLayers finds all layer directories in workDir containing docker-compose files
func DiscoverLayers(workDir string) ([]string, map[string]string, error) {
	composeMap := make(map[string]string)
	discovered := make(map[string]bool)

	// Check root level compose as fallback
	rootCandidates := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}
	for _, c := range rootCandidates {
		p := filepath.Join(workDir, c)
		if _, err := os.Stat(p); err == nil {
			composeMap["."] = p
			discovered["."] = true
			break
		}
	}

	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read directory %s: %w", workDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		layerDir := filepath.Join(workDir, name)
		for _, c := range rootCandidates {
			composePath := filepath.Join(layerDir, c)
			if _, err := os.Stat(composePath); err == nil {
				composeMap[name] = composePath
				discovered[name] = true
				break
			}
		}
	}

	// Order layers according to canonical dependency order, then custom layers alphabetically
	var orderedLayers []string
	for _, l := range defaultLayerOrder {
		if discovered[l] {
			orderedLayers = append(orderedLayers, l)
		}
	}

	var customLayers []string
	for l := range discovered {
		isDefault := false
		for _, dl := range defaultLayerOrder {
			if l == dl {
				isDefault = true
				break
			}
		}
		if !isDefault && l != "." {
			customLayers = append(customLayers, l)
		}
	}
	sort.Strings(customLayers)
	orderedLayers = append(orderedLayers, customLayers...)

	if discovered["."] {
		orderedLayers = append(orderedLayers, ".")
	}

	return orderedLayers, composeMap, nil
}

// ResolveTargets resolves target patterns to matching services in the workDir
func ResolveTargets(workDir string, targets []string) ([]ResolvedTarget, error) {
	orderedLayers, composeMap, err := DiscoverLayers(workDir)
	if err != nil {
		return nil, err
	}

	// Load all services across layers
	type layerServices struct {
		layerName   string
		composePath string
		services    map[string]ComposeService
	}

	var allLayers []layerServices
	for _, l := range orderedLayers {
		cPath := composeMap[l]
		cfg, err := ParseComposeFile(cPath)
		if err != nil {
			return nil, err
		}
		allLayers = append(allLayers, layerServices{
			layerName:   l,
			composePath: cPath,
			services:    cfg.Services,
		})
	}

	// If no targets supplied: return all services in dependency order
	if len(targets) == 0 {
		var results []ResolvedTarget
		for _, ls := range allLayers {
			// Sort service names within layer for deterministic order
			var sNames []string
			for sName := range ls.services {
				sNames = append(sNames, sName)
			}
			sort.Strings(sNames)

			for _, sName := range sNames {
				s := ls.services[sName]
				results = append(results, ResolvedTarget{
					LayerName:     ls.layerName,
					ComposePath:   ls.composePath,
					ServiceName:   sName,
					ContainerName: s.ContainerName,
					Image:         s.Image,
					Labels:        s.ParsedLabels,
				})
			}
		}
		return results, nil
	}

	var results []ResolvedTarget
	seen := make(map[string]bool)

	for _, target := range targets {
		matchedTarget := false

		// 1. Layer Target or Scoped Service (/layer or /layer/service)
		if strings.HasPrefix(target, "/") {
			trimmed := strings.TrimPrefix(target, "/")
			parts := strings.SplitN(trimmed, "/", 2)
			targetLayer := parts[0]
			var targetService string
			if len(parts) == 2 {
				targetService = parts[1]
			}

			for _, ls := range allLayers {
				if ls.layerName == targetLayer {
					var sNames []string
					for sName := range ls.services {
						sNames = append(sNames, sName)
					}
					sort.Strings(sNames)

					for _, sName := range sNames {
						if targetService != "" && sName != targetService {
							continue
						}
						s := ls.services[sName]
						key := ls.layerName + "/" + sName
						if !seen[key] {
							seen[key] = true
							results = append(results, ResolvedTarget{
								LayerName:     ls.layerName,
								ComposePath:   ls.composePath,
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
				return nil, fmt.Errorf("target layer or service not found: %s", target)
			}
			continue
		}

		// 2. Bare Name or Double Dot Wildcard Matching
		for _, ls := range allLayers {
			var sNames []string
			for sName := range ls.services {
				sNames = append(sNames, sName)
			}
			sort.Strings(sNames)

			for _, sName := range sNames {
				if MatchWildcard(target, sName) {
					s := ls.services[sName]
					key := ls.layerName + "/" + sName
					if !seen[key] {
						seen[key] = true
						results = append(results, ResolvedTarget{
							LayerName:     ls.layerName,
							ComposePath:   ls.composePath,
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
			return nil, fmt.Errorf("no matching services found for target: %s", target)
		}
	}

	return results, nil
}
