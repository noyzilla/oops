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
