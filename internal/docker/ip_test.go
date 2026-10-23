package docker

import (
	"testing"
)

func TestGetContainerIP_NonExistent(t *testing.T) {
	// Querying a non-existent container should gracefully return empty string
	ip := GetContainerIP("non_existent_container_12345", "oops_edge")
	if ip != "" {
		t.Errorf("expected empty string for non-existent container, got %q", ip)
	}
}

func TestGetOopsContainerIP(t *testing.T) {
	// Should not panic, either returns container IP or local/fallback IP or empty string
	ip := GetOopsContainerIP()
	t.Logf("GetOopsContainerIP returned: %q", ip)
}

