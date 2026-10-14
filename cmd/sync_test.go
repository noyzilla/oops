package cmd

import (
	"testing"
)

func TestParsePushPullArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantStack string
		wantName  string
	}{
		{
			name:      "No args",
			args:      []string{},
			wantStack: "",
			wantName:  "",
		},
		{
			name:      "Only remote with @",
			args:      []string{"@prod"},
			wantStack: "",
			wantName:  "prod",
		},
		{
			name:      "Only bare remote",
			args:      []string{"staging"},
			wantStack: "",
			wantName:  "staging",
		},
		{
			name:      "Stack and remote",
			args:      []string{"/crm", "@prod"},
			wantStack: "crm",
			wantName:  "prod",
		},
		{
			name:      "Remote and stack inverted",
			args:      []string{"@prod", "/crm"},
			wantStack: "crm",
			wantName:  "prod",
		},
		{
			name:      "Only stack",
			args:      []string{"/crm"},
			wantStack: "crm",
			wantName:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStack, gotName := parsePushPullArgs(tt.args)
			if gotStack != tt.wantStack || gotName != tt.wantName {
				t.Errorf("parsePushPullArgs(%v) = (%q, %q), want (%q, %q)",
					tt.args, gotStack, gotName, tt.wantStack, tt.wantName)
			}
		})
	}
}
