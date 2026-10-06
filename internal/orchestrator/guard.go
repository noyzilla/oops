package orchestrator

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strings"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/storage"
)

// edgeStackName is never blocked: the webhook server and DNS live in the edge stack
const edgeStackName = "edge"

// BlockedService is a target the storage guard refuses to start
type BlockedService struct {
	Target docker.ResolvedTarget
	Reason string
}

// BlockedError is returned after unaffected services ran but some were blocked
type BlockedError struct {
	Blocked []BlockedService
}

func (e *BlockedError) Error() string {
	var names []string
	for _, b := range e.Blocked {
		names = append(names, fmt.Sprintf("%s/%s", b.Target.StackName, b.Target.ServiceName))
	}
	return fmt.Sprintf("storage guard blocked %d service(s): %s", len(e.Blocked), strings.Join(names, ", "))
}

func targetKey(t docker.ResolvedTarget) string {
	return t.ComposePath + "|" + t.ServiceName
}

// BindSources returns resolved bind-mount sources per service from docker compose config
func BindSources(composePath string) map[string][]string {
	args := []string{"compose"}
	if envFile := FindEnvFile(composePath); envFile != "" {
		args = append(args, "--env-file", envFile)
	}
	args = append(args, "-f", composePath, "config", "--format", "json")

	out, err := exec.Command("docker", args...).Output()
	if err != nil {
		log.Printf("Warning: storage guard could not read compose config for %s: %v", composePath, err)
		return nil
	}
	return parseBindSources(out)
}

func parseBindSources(raw []byte) map[string][]string {
	var cfg struct {
		Services map[string]struct {
			Volumes []struct {
				Type   string `json:"type"`
				Source string `json:"source"`
			} `json:"volumes"`
		} `json:"services"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}
	res := make(map[string][]string)
	for name, svc := range cfg.Services {
		for _, v := range svc.Volumes {
			if v.Type == "bind" && v.Source != "" {
				res[name] = append(res[name], v.Source)
			}
		}
	}
	return res
}

// priorityRanks maps each target key to the index of the first matching priority entry
func priorityRanks(workDir string, entries []string) map[string]int {
	ranks := make(map[string]int)
	for i, entry := range entries {
		resolved, err := docker.ResolveTargets(workDir, []string{entry})
		if err != nil {
			continue
		}
		for _, t := range resolved {
			if _, seen := ranks[targetKey(t)]; !seen {
				ranks[targetKey(t)] = i
			}
		}
	}
	return ranks
}

// planGuard decides which targets are allowed and blocked given the detected Bad Links
func planGuard(targets []docker.ResolvedTarget, check func(source string) *storage.Problem,
	binds func(composePath string) map[string][]string, ranks map[string]int, unlisted int) (allowed []docker.ResolvedTarget, blocked []BlockedService) {

	rank := func(t docker.ResolvedTarget) int {
		if r, ok := ranks[targetKey(t)]; ok {
			return r
		}
		return unlisted
	}

	affected := make(map[string]string)
	bindCache := make(map[string]map[string][]string)
	minRank := unlisted + 1
	for _, t := range targets {
		if _, ok := bindCache[t.ComposePath]; !ok {
			bindCache[t.ComposePath] = binds(t.ComposePath)
		}
		for _, src := range bindCache[t.ComposePath][t.ServiceName] {
			if p := check(src); p != nil {
				affected[targetKey(t)] = fmt.Sprintf("%s -> %s: %s", p.Path, p.Target, p.Reason)
				break
			}
		}
		if _, hit := affected[targetKey(t)]; hit && rank(t) < minRank {
			minRank = rank(t)
		}
	}

	for _, t := range targets {
		if t.StackName == edgeStackName {
			allowed = append(allowed, t)
			continue
		}
		if reason, hit := affected[targetKey(t)]; hit {
			blocked = append(blocked, BlockedService{Target: t, Reason: reason})
			continue
		}
		if len(affected) > 0 && rank(t) > minRank {
			blocked = append(blocked, BlockedService{Target: t, Reason: "lower priority than a service with unhealthy storage"})
			continue
		}
		allowed = append(allowed, t)
	}

	// edge first so DNS and the webhook server come up before everything else
	sort.SliceStable(allowed, func(i, j int) bool {
		return allowed[i].StackName == edgeStackName && allowed[j].StackName != edgeStackName
	})
	return allowed, blocked
}

// StoragePrefixes returns the configured mount prefixes or the defaults when unset
func StoragePrefixes(workDir string) []string {
	cfg, err := docker.LoadOopsConfig(workDir)
	if err != nil || cfg.Storage.MountPrefixes == nil {
		return storage.DefaultMountPrefixes
	}
	return cfg.Storage.MountPrefixes
}

// CheckBackupDir refuses a backup directory that is a dead link or sits on the OS disk under a mount prefix.
// It is deliberately independent of container management.
func CheckBackupDir(workDir, backupDir string) error {
	if p := storage.NewChecker().CheckPath(backupDir, StoragePrefixes(workDir), true); p != nil {
		return fmt.Errorf("backup directory %s is unhealthy: %s -> %s: %s", backupDir, p.Path, p.Target, p.Reason)
	}
	return nil
}

// Finding is an unhealthy bind source or backup directory found by InspectBox
type Finding struct {
	Stack   string
	Source  string
	Problem storage.Problem
}

// InspectBox checks every bind source of every stack plus the backup directory
func InspectBox(workDir, backupDir string) ([]Finding, error) {
	_, composeMap, err := docker.DiscoverStacks(workDir)
	if err != nil {
		return nil, err
	}
	checker := storage.NewChecker()
	prefixes := StoragePrefixes(workDir)

	var stacks []string
	for name := range composeMap {
		stacks = append(stacks, name)
	}
	sort.Strings(stacks)

	var findings []Finding
	seen := make(map[string]bool)
	for _, stack := range stacks {
		binds := BindSources(composeMap[stack])
		var services []string
		for svc := range binds {
			services = append(services, svc)
		}
		sort.Strings(services)
		for _, svc := range services {
			for _, src := range binds[svc] {
				key := stack + "|" + src
				if seen[key] {
					continue
				}
				seen[key] = true
				if p := checker.CheckPath(src, prefixes, false); p != nil {
					findings = append(findings, Finding{Stack: stack + "/" + svc, Source: src, Problem: *p})
				}
			}
		}
	}
	if backupDir != "" {
		if p := checker.CheckPath(backupDir, prefixes, true); p != nil {
			findings = append(findings, Finding{Stack: "backup", Source: backupDir, Problem: *p})
		}
	}
	return findings, nil
}

// applyGuard filters targets through the storage guard. The returned error is a *BlockedError
// when services were blocked; callers run the allowed targets first and then return it.
func (o *Orchestrator) applyGuard(targets []docker.ResolvedTarget) ([]docker.ResolvedTarget, error) {
	if o.WorkDir == "" || len(targets) == 0 {
		return targets, nil
	}

	cfg, err := docker.LoadOopsConfig(o.WorkDir)
	if err != nil {
		log.Printf("Warning: storage guard skipped, cannot load oops.yml: %v", err)
		return targets, nil
	}

	checker := storage.NewChecker()
	prefixes := StoragePrefixes(o.WorkDir)
	logged := make(map[string]bool)
	check := func(source string) *storage.Problem {
		p := checker.CheckPath(source, prefixes, false)
		if p != nil && !logged[p.Path] {
			logged[p.Path] = true
			log.Printf("ERROR: storage path %s -> %s is unhealthy: %s", p.Path, p.Target, p.Reason)
		}
		return p
	}

	ranks := priorityRanks(o.WorkDir, cfg.Priority)
	allowed, blocked := planGuard(targets, check, BindSources, ranks, len(cfg.Priority))
	if len(blocked) == 0 {
		return allowed, nil
	}
	for _, b := range blocked {
		log.Printf("ERROR: blocked %s/%s (%s)", b.Target.StackName, b.Target.ServiceName, b.Reason)
	}
	return allowed, &BlockedError{Blocked: blocked}
}
