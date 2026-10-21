package cmd

import (
	"testing"
)

func TestParseRxInvocation(t *testing.T) {
	tmpDir := t.TempDir()
	tests := []struct {
		name       string
		args       []string
		wantServer string
		wantCmds   []string
	}{
		{
			name:       "Implicit default server ps command",
			args:       []string{"rx", "ps"},
			wantServer: "prod",
			wantCmds:   []string{"ps"},
		},
		{
			name:       "Explicit server @lima-debian",
			args:       []string{"rx", "@lima-debian", "logs", "app", "-f"},
			wantServer: "lima-debian",
			wantCmds:   []string{"logs", "app", "-f"},
		},
		{
			name:       "Explicit server @prod",
			args:       []string{"rx", "@prod", "up", "/db"},
			wantServer: "prod",
			wantCmds:   []string{"up", "/db"},
		},
		{
			name:       "Explicit server @staging",
			args:       []string{"rx", "@staging", "db", "mysql", "list"},
			wantServer: "staging",
			wantCmds:   []string{"db", "mysql", "list"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotServer, gotCmds := parseRxInvocation(tt.args, tmpDir)
			if gotServer != tt.wantServer {
				t.Errorf("parseRxInvocation(%v) server = %q, want %q", tt.args, gotServer, tt.wantServer)
			}
			if len(gotCmds) != len(tt.wantCmds) {
				t.Errorf("parseRxInvocation(%v) cmds = %v, want %v", tt.args, gotCmds, tt.wantCmds)
			} else {
				for i := range gotCmds {
					if gotCmds[i] != tt.wantCmds[i] {
						t.Errorf("parseRxInvocation(%v) cmds[%d] = %q, want %q", tt.args, i, gotCmds[i], tt.wantCmds[i])
					}
				}
			}
		})
	}
}
