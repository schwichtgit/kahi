package logging

import "testing"

func TestStripANSI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"\033[31mERROR\033[0m", "ERROR"},
		{"\033[1;32mOK\033[0m", "OK"},
		{"no ansi here", "no ansi here"},
		{"\033[2J\033[H", ""},
		{"\033[38;5;196mred\033[0m text", "red text"},
	}

	for _, tt := range tests {
		got := string(StripANSI([]byte(tt.input)))
		if got != tt.want {
			t.Errorf("StripANSI(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
