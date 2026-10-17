package docker

import "testing"

func TestNormalizeGitURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTTPS with .git",
			input:    "https://github.com/noyzilla/oops.git",
			expected: "https://github.com/noyzilla/oops",
		},
		{
			name:     "HTTPS with trailing slash",
			input:    "https://github.com/noyzilla/oops/",
			expected: "https://github.com/noyzilla/oops",
		},
		{
			name:     "SSH format with .git",
			input:    "git@github.com:noyzilla/oops.git",
			expected: "git@github.com:noyzilla/oops",
		},
		{
			name:     "Local file path",
			input:    "/tmp/testrepo",
			expected: "/tmp/testrepo",
		},
		{
			name:     "With spaces",
			input:    "  https://github.com/noyzilla/oops.git  ",
			expected: "https://github.com/noyzilla/oops",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeGitURL(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeGitURL(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBuildTargetComposeArgs(t *testing.T) {
	target := ResolvedTarget{
		StackName:    "edge",
		ComposePath:  "/opt/oopsbox/compose.yaml",
		ComposeFiles: []string{"/opt/oopsbox/compose.yaml", "/opt/oopsbox/compose.edge.yaml"},
		ServiceName:  "caddy-proxy",
	}

	got := BuildTargetComposeArgs(target, "up", "-d", "--no-deps", "caddy-proxy")
	want := []string{
		"compose",
		"-f", "/opt/oopsbox/compose.yaml",
		"-f", "/opt/oopsbox/compose.edge.yaml",
		"up", "-d", "--no-deps", "caddy-proxy",
	}

	if len(got) != len(want) {
		t.Fatalf("BuildTargetComposeArgs() len = %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("BuildTargetComposeArgs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

