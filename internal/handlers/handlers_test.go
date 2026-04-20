package handlers

import (
	"testing"
)

func TestSanitizeSlug(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple",
			input: "My Agent",
			want:  "MyAgent",
		},
		{
			name:  "with spaces",
			input: "my agent 1",
			want:  "myagent1",
		},
		{
			name:  "special chars removed",
			input: "agent@#$%123",
			want:  "agent123",
		},
		{
			name:  "only valid chars",
			input: "my-agent-1",
			want:  "my-agent-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeSlug(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeSlug() = %q, want %q", got, tt.want)
			}
		})
	}
}
