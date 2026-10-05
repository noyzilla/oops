package cmd

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

func TestFormatContainerPorts(t *testing.T) {
	tests := []struct {
		name     string
		inspect  *container.InspectResponse
		expected string
	}{
		{
			name:     "Nil or empty network settings",
			inspect:  &container.InspectResponse{},
			expected: "-",
		},
		{
			name: "All-interface TCP deduplicated and internal port without parentheses",
			inspect: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"80/tcp": []nat.PortBinding{
								{HostIP: "0.0.0.0", HostPort: "80"},
								{HostIP: "::", HostPort: "80"},
							},
							"443/tcp": []nat.PortBinding{
								{HostIP: "0.0.0.0", HostPort: "443"},
								{HostIP: "::", HostPort: "443"},
							},
							"2019/tcp": nil,
						},
					},
				},
			},
			expected: "*:80, *:443, 2019",
		},
		{
			name: "All-interface UDP port formatting without redundant container port",
			inspect: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"53/udp": []nat.PortBinding{
								{HostIP: "0.0.0.0", HostPort: "53"},
							},
						},
					},
				},
			},
			expected: "*:53/udp",
		},
		{
			name: "Multiple host ports mapped to same container port",
			inspect: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"9000/tcp": []nat.PortBinding{
								{HostIP: "0.0.0.0", HostPort: "9001"},
								{HostIP: "0.0.0.0", HostPort: "9002"},
								{HostIP: "0.0.0.0", HostPort: "9003"},
							},
						},
					},
				},
			},
			expected: "*:[9001,9002,9003]->9000",
		},
		{
			name: "Loopback 127.0.0.1 Host IP binding with #: prefix and omitted same port",
			inspect: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"8080/tcp": []nat.PortBinding{
								{HostIP: "127.0.0.1", HostPort: "8080"},
							},
						},
					},
				},
			},
			expected: "#:8080",
		},
		{
			name: "Loopback 127.0.0.1 Host IP binding with different ports",
			inspect: &container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{
					NetworkSettingsBase: container.NetworkSettingsBase{
						Ports: nat.PortMap{
							"8000/tcp": []nat.PortBinding{
								{HostIP: "127.0.0.1", HostPort: "8080"},
							},
						},
					},
				},
			},
			expected: "#:8080->8000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatContainerPorts(tt.inspect)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
