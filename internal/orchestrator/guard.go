package orchestrator

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
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

// bindSources returns resolved bind-mount sources per service from docker compose config
func bindSources(composePath string) map[string][]string {
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
func planGuard(targets []docker.ResolvedTarget, problems []storage.Problem,
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
			for _, p := range problems {
				if p.PassesThrough(src) {
					affected[targetKey(t)] = fmt.Sprintf("%s -> %s: %s", p.LinkPath, p.Target, p.Reason)
				}
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

	prefixes := cfg.Storage.MountPrefixes
	if prefixes == nil {
		prefixes = storage.DefaultMountPrefixes
	}

	// Bind sources from compose are absolute, so link paths must be absolute to compare
	boxDir, err := filepath.Abs(o.WorkDir)
	if err != nil {
		boxDir = o.WorkDir
	}

	problems := storage.NewChecker().Inspect(boxDir, prefixes)
	if len(problems) == 0 {
		return targets, nil
	}
	for _, p := range problems {
		log.Printf("ERROR: storage link %s -> %s is unhealthy: %s", p.LinkPath, p.Target, p.Reason)
	}

	ranks := priorityRanks(o.WorkDir, cfg.Priority)
	allowed, blocked := planGuard(targets, problems, bindSources, ranks, len(cfg.Priority))
	if len(blocked) == 0 {
		return allowed, nil
	}
	for _, b := range blocked {
		log.Printf("ERROR: blocked %s/%s (%s)", b.Target.StackName, b.Target.ServiceName, b.Reason)
	}
	return allowed, &BlockedError{Blocked: blocked}
}
