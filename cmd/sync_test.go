package cmd

import (
	"testing"
)

func TestParseSyncRemote(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "No args",
			args: []string{},
			want: "",
		},
		{
			name: "Prefix with @",
			args: []string{"@prod"},
			want: "prod",
		},
		{
			name: "Bare remote name",
			args: []string{"staging"},
			want: "staging",
		},
		{
			name: "Prefix with @ among flags",
			args: []string{"-v", "@prod"},
			want: "prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSyncRemote(tt.args)
			if got != tt.want {
				t.Errorf("parseSyncRemote(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
