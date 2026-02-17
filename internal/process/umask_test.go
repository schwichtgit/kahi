package process

import (
	"testing"
)

func TestParseUmaskEmpty(t *testing.T) {
	val, err := ParseUmask("")
	if err != nil {
		t.Fatal(err)
	}
	if val != -1 {
		t.Fatalf("val = %d, want -1 (inherit)", val)
	}
}

func TestParseUmaskOctal(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"022", 0o22},
		{"0022", 0o22},
		{"0755", 0o755},
		{"0644", 0o644},
		{"0", 0},
		{"0777", 0o777},
	}

	for _, tt := range tests {
		val, err := ParseUmask(tt.input)
		if err != nil {
			t.Fatalf("ParseUmask(%q): %v", tt.input, err)
		}
		if val != tt.want {
			t.Fatalf("ParseUmask(%q) = %o, want %o", tt.input, val, tt.want)
		}
	}
}

func TestParseUmaskInvalid(t *testing.T) {
	_, err := ParseUmask("999") // 9 is not an octal digit
	if err == nil {
		t.Fatal("expected error for non-octal string")
	}
}

func TestParseUmaskOutOfRange(t *testing.T) {
	_, err := ParseUmask("1000") // > 0777
	if err == nil {
		t.Fatal("expected error for out of range")
	}
}

func TestApplyUmaskNegative(t *testing.T) {
	prev := ApplyUmask(-1)
	if prev != 0 {
		t.Fatalf("ApplyUmask(-1) = %d, want 0 (no-op)", prev)
	}
}

func TestApplyUmaskValid(t *testing.T) {
	// Save and restore umask.
	old := ApplyUmask(0o022)
	restored := ApplyUmask(old)
	if restored != 0o022 {
		t.Fatalf("restored umask = %o, want 022", restored)
	}
}
