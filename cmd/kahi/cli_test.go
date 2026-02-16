package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	for _, sub := range []string{"daemon", "ctl", "migrate", "version", "init", "hash-password", "completion"} {
		if !strings.Contains(out, sub) {
			t.Errorf("help output missing subcommand %q", sub)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	for _, want := range []string{"kahi", "commit:", "built:", "go:", "os/arch:", "fips:"} {
		if !strings.Contains(out, want) {
			t.Errorf("version output missing %q", want)
		}
	}
}

func TestUnknownSubcommand(t *testing.T) {
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}
