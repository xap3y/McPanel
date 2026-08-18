package mcserver

import "testing"

func TestNormalizeConsoleLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"plain prompt", ">\r\n", "", false},
		{"command echo", "> plugins\r\n", "", false},
		{"backspace redraw", "> p\bpl\b\bpluginplugins\r\n", "", false},
		{"prompt overwritten", "\x1b[?1h\x1b=> \r\x1b[K[23:49:56 INFO]: Ready\r\n", "[23:49:56 INFO]: Ready", true},
		{"true color", "[INFO]: \x1b[38;2;52;159;218mInfo\x1b[0m\r\n", "[INFO]: \x1b[38;2;52;159;218mInfo\x1b[0m", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizeConsoleLine(tt.in)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("normalizeConsoleLine() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}
