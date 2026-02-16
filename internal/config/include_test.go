package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveIncludesGlob(t *testing.T) {
	dir := t.TempDir()

	// Main config.
	mainCfg := &Config{
		Include:  []string{filepath.Join(dir, "conf.d/*.toml")},
		Programs: make(map[string]ProgramConfig),
	}

	// Create conf.d directory with files.
	confDir := filepath.Join(dir, "conf.d")
	os.MkdirAll(confDir, 0755)

	webCfg := `[programs.web]
command = "/usr/bin/web"
`
	apiCfg := `[programs.api]
command = "/usr/bin/api"
`
	os.WriteFile(filepath.Join(confDir, "01-web.toml"), []byte(webCfg), 0644)
	os.WriteFile(filepath.Join(confDir, "02-api.toml"), []byte(apiCfg), 0644)

	warnings, err := ResolveIncludes(mainCfg, dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(mainCfg.Programs) != 2 {
		t.Fatalf("expected 2 programs, got %d", len(mainCfg.Programs))
	}

	if _, ok := mainCfg.Programs["web"]; !ok {
		t.Fatal("missing program 'web'")
	}
	if _, ok := mainCfg.Programs["api"]; !ok {
		t.Fatal("missing program 'api'")
	}

	_ = warnings
}

func TestResolveIncludesNoMatches(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Include:  []string{filepath.Join(dir, "nonexistent/*.toml")},
		Programs: make(map[string]ProgramConfig),
	}

	warnings, err := ResolveIncludes(cfg, dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(warnings) == 0 {
		t.Fatal("expected warning for no-match pattern")
	}
}

func TestResolveIncludesRelativePath(t *testing.T) {
	dir := t.TempDir()

	// Create a relative include path.
	confDir := filepath.Join(dir, "conf.d")
	os.MkdirAll(confDir, 0755)

	webCfg := `[programs.web]
command = "/usr/bin/web"
`
	os.WriteFile(filepath.Join(confDir, "web.toml"), []byte(webCfg), 0644)

	cfg := &Config{
		Include:  []string{"conf.d/*.toml"},
		Programs: make(map[string]ProgramConfig),
	}

	_, err := ResolveIncludes(cfg, dir)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := cfg.Programs["web"]; !ok {
		t.Fatal("missing program 'web' from relative include")
	}
}

func TestResolveIncludesSyntaxError(t *testing.T) {
	dir := t.TempDir()
	confDir := filepath.Join(dir, "conf.d")
	os.MkdirAll(confDir, 0755)

	os.WriteFile(filepath.Join(confDir, "bad.toml"), []byte("[[invalid"), 0644)

	cfg := &Config{
		Include:  []string{filepath.Join(dir, "conf.d/*.toml")},
		Programs: make(map[string]ProgramConfig),
	}

	_, err := ResolveIncludes(cfg, dir)
	if err == nil {
		t.Fatal("expected error for syntax error in included file")
	}
}

func TestResolveIncludesDuplicateProgram(t *testing.T) {
	dir := t.TempDir()
	confDir := filepath.Join(dir, "conf.d")
	os.MkdirAll(confDir, 0755)

	webCfg := `[programs.web]
command = "/usr/bin/web"
`
	os.WriteFile(filepath.Join(confDir, "01.toml"), []byte(webCfg), 0644)
	os.WriteFile(filepath.Join(confDir, "02.toml"), []byte(webCfg), 0644)

	cfg := &Config{
		Include:  []string{filepath.Join(dir, "conf.d/*.toml")},
		Programs: make(map[string]ProgramConfig),
	}

	_, err := ResolveIncludes(cfg, dir)
	if err == nil {
		t.Fatal("expected error for duplicate program name")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error = %q, want duplicate program error", err.Error())
	}
}

func TestResolveIncludesClearsIncludeField(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		Include:  []string{filepath.Join(dir, "nonexistent/*.toml")},
		Programs: make(map[string]ProgramConfig),
	}

	_, _ = ResolveIncludes(cfg, dir)

	if cfg.Include != nil {
		t.Fatal("include field should be cleared after resolution")
	}
}

func TestValidateGroupReferencesEmpty(t *testing.T) {
	cfg := &Config{
		Programs: map[string]ProgramConfig{
			"web": {Command: "/usr/bin/web"},
		},
		Groups: map[string]GroupConfig{
			"empty": {Programs: []string{}},
		},
	}

	errs := validateGroupReferences(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for empty group")
	}
}

func TestValidateGroupReferencesNonexistent(t *testing.T) {
	cfg := &Config{
		Programs: map[string]ProgramConfig{},
		Groups: map[string]GroupConfig{
			"mygroup": {Programs: []string{"nonexistent"}},
		},
	}

	errs := validateGroupReferences(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for nonexistent program reference")
	}
}

func TestValidateGroupReferencesMultipleGroups(t *testing.T) {
	cfg := &Config{
		Programs: map[string]ProgramConfig{
			"web": {Command: "/usr/bin/web"},
		},
		Groups: map[string]GroupConfig{
			"group1": {Programs: []string{"web"}},
			"group2": {Programs: []string{"web"}},
		},
	}

	errs := validateGroupReferences(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for program in multiple groups")
	}
}
