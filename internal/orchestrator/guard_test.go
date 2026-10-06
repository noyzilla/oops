package orchestrator

import (
	"testing"

	"github.com/noyzilla/oops/internal/docker"
	"github.com/noyzilla/oops/internal/storage"
)

func tgt(stack, svc string) docker.ResolvedTarget {
	return docker.ResolvedTarget{StackName: stack, ServiceName: svc, ComposePath: "/box/stacks/" + stack + "/compose.yml"}
}

func names(ts []docker.ResolvedTarget) []string {
	var r []string
	for _, t := range ts {
		r = append(r, t.StackName+"/"+t.ServiceName)
	}
	return r
}

func TestPlanGuardBlocksAffectedAndLowerPriority(t *testing.T) {
	targets := []docker.ResolvedTarget{
		tgt("apps", "web"), tgt("db", "mysql"), tgt("edge", "caddy"), tgt("db", "redis"), tgt("tool", "adminer"),
	}
	problems := []storage.Problem{{LinkPath: "/box/data", Target: "/mnt/x", Reason: "dead"}}
	binds := func(compose string) map[string][]string {
		return map[string][]string{"mysql": {"/box/data/mysql"}, "redis": {"/box/other"}}
	}
	// edge=0, db=1, apps=2; tool unlisted
	ranks := map[string]int{
		targetKey(tgt("edge", "caddy")): 0,
		targetKey(tgt("db", "mysql")):   1,
		targetKey(tgt("db", "redis")):   1,
		targetKey(tgt("apps", "web")):   2,
	}

	allowed, blocked := planGuard(targets, problems, binds, ranks, 3)

	gotAllowed := names(allowed)
	if len(gotAllowed) != 2 || gotAllowed[0] != "edge/caddy" || gotAllowed[1] != "db/redis" {
		t.Fatalf("allowed (edge first, same-priority peer kept): %v", gotAllowed)
	}
	if len(blocked) != 3 {
		t.Fatalf("expected mysql + web + adminer blocked, got %d", len(blocked))
	}
}

func TestPlanGuardEdgeNeverBlocked(t *testing.T) {
	targets := []docker.ResolvedTarget{tgt("edge", "caddy"), tgt("db", "mysql")}
	problems := []storage.Problem{{LinkPath: "/box/data", Target: "/mnt/x", Reason: "dead"}}
	binds := func(string) map[string][]string {
		return map[string][]string{"caddy": {"/box/data/caddy"}, "mysql": {"/box/data/mysql"}}
	}
	allowed, blocked := planGuard(targets, problems, binds, nil, 0)
	if len(allowed) != 1 || allowed[0].StackName != "edge" || len(blocked) != 1 {
		t.Fatalf("edge must stay allowed: allowed=%v blocked=%d", names(allowed), len(blocked))
	}
}

func TestPlanGuardNoAffectedAllowsAll(t *testing.T) {
	targets := []docker.ResolvedTarget{tgt("apps", "web"), tgt("db", "mysql")}
	problems := []storage.Problem{{LinkPath: "/box/data", Reason: "dead"}}
	binds := func(string) map[string][]string { return nil }
	allowed, blocked := planGuard(targets, problems, binds, nil, 0)
	if len(allowed) != 2 || len(blocked) != 0 {
		t.Fatalf("unrelated services must run: %v", names(allowed))
	}
}

func TestParseBindSources(t *testing.T) {
	raw := []byte(`{"services":{"a":{"volumes":[{"type":"bind","source":"/box/data/a"},{"type":"volume","source":"v"}]}}}`)
	got := parseBindSources(raw)
	if len(got["a"]) != 1 || got["a"][0] != "/box/data/a" {
		t.Fatalf("unexpected %v", got)
	}
}
