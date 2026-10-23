package dns

import (
	"reflect"
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestExtractContainerHostnames_SingleAndMulti(t *testing.T) {
	tests := []struct {
		name     string
		inspect  *container.InspectResponse
		expected []string
	}{
		{
			name:     "nil inspect",
			inspect:  nil,
			expected: nil,
		},
		{
			name: "from Config.Hostname only",
			inspect: &container.InspectResponse{
				Config: &container.Config{
					Hostname: "my-service.oops",
				},
			},
			expected: []string{"my-service.oops"},
		},
		{
			name: "from label single hostname",
			inspect: &container.InspectResponse{
				Config: &container.Config{
					Hostname: "ignore-me",
					Labels: map[string]string{
						"oops.dns.hostname": "tunnel-b9tpe.oops",
					},
				},
			},
			expected: []string{"tunnel-b9tpe.oops", "ignore-me"},
		},
		{
			name: "from label comma separated hostnames",
			inspect: &container.InspectResponse{
				Config: &container.Config{
					Labels: map[string]string{
						"oops.dns.hostname": "tunnel-b9tpe.oops, b9tpe.oops",
					},
				},
			},
			expected: []string{"tunnel-b9tpe.oops", "b9tpe.oops"},
		},
		{
			name: "ignore localhost and duplicates",
			inspect: &container.InspectResponse{
				Config: &container.Config{
					Hostname: "localhost",
					Labels: map[string]string{
						"oops.dns.hostname": "tunnel-b9tpe.oops, TUNNEL-B9TPE.OOPS, localhost",
					},
				},
			},
			expected: []string{"tunnel-b9tpe.oops"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractContainerHostnames(tt.inspect)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ExtractContainerHostnames() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExtractContainerDNSMappings(t *testing.T) {
	inspect := &container.InspectResponse{
		Config: &container.Config{
			Labels: map[string]string{
				"oops.dns.mappings": "mysql1.b9tpe.tunnel.oops=172.18.0.21, mysql2.b9tpe.tunnel.oops=172.18.0.22",
			},
		},
	}

	mappings := ExtractContainerDNSMappings(inspect)
	if len(mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %d", len(mappings))
	}
	if mappings["mysql1.b9tpe.tunnel.oops"].String() != "172.18.0.21" {
		t.Errorf("expected 172.18.0.21, got %v", mappings["mysql1.b9tpe.tunnel.oops"])
	}
	if mappings["mysql2.b9tpe.tunnel.oops"].String() != "172.18.0.22" {
		t.Errorf("expected 172.18.0.22, got %v", mappings["mysql2.b9tpe.tunnel.oops"])
	}
}

