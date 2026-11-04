package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig(envMap(map[string]string{"OPENROUTER_MANAGEMENT_KEY": testKey}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Addr != defaultAddr {
		t.Fatalf("addr = %q", cfg.Addr)
	}
	if cfg.APIURL != defaultAPIURL {
		t.Fatalf("api url = %q", cfg.APIURL)
	}
	if cfg.Interval != defaultInterval {
		t.Fatalf("interval = %s", cfg.Interval)
	}
	if cfg.Timeout != defaultTimeout {
		t.Fatalf("timeout = %s", cfg.Timeout)
	}
	if cfg.StaleAfter != defaultInterval*defaultStaleMul {
		t.Fatalf("stale after = %s", cfg.StaleAfter)
	}
	if cfg.KeySource != "env" {
		t.Fatalf("key source = %q", cfg.KeySource)
	}
}

func TestLoadConfigRequiresManagementKey(t *testing.T) {
	_, err := LoadConfig(envMap(map[string]string{}))
	if !errors.Is(err, ErrMissingManagementKey) {
		t.Fatalf("expected ErrMissingManagementKey, got %v", err)
	}
	if _, err := LoadConfig(envMap(map[string]string{"OPENROUTER_MANAGEMENT_KEY": "   "})); !errors.Is(err, ErrMissingManagementKey) {
		t.Fatalf("expected ErrMissingManagementKey for whitespace key, got %v", err)
	}
}

func TestLoadConfigFromSecretFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key")
	if err := os.WriteFile(path, []byte(testKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(envMap(map[string]string{
		// File must win over the raw env value.
		"OPENROUTER_MANAGEMENT_KEY":      "sk-or-v1-environment-value",
		"OPENROUTER_MANAGEMENT_KEY_FILE": path,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ManagementKey != testKey {
		t.Fatal("secret file was not preferred over the environment value")
	}
	if cfg.KeySource != "file" {
		t.Fatalf("key source = %q", cfg.KeySource)
	}
}

func TestLoadConfigSecretFileErrors(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte("  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"missing file": filepath.Join(dir, "nope"),
		"empty file":   empty,
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := LoadConfig(envMap(map[string]string{"OPENROUTER_MANAGEMENT_KEY_FILE": path}))
			if err == nil {
				t.Fatal("expected an error")
			}
			if errors.Is(err, ErrMissingManagementKey) {
				t.Fatal("file errors must be distinguishable from a missing key")
			}
		})
	}
}

func TestLoadConfigValidation(t *testing.T) {
	base := map[string]string{"OPENROUTER_MANAGEMENT_KEY": testKey}
	cases := []struct {
		name  string
		extra map[string]string
	}{
		{"interval too low", map[string]string{"OPENROUTER_EXPORTER_INTERVAL": "5s"}},
		{"interval unparsable", map[string]string{"OPENROUTER_EXPORTER_INTERVAL": "soon"}},
		{"timeout zero", map[string]string{"OPENROUTER_EXPORTER_TIMEOUT": "0s"}},
		{"timeout too high", map[string]string{"OPENROUTER_EXPORTER_TIMEOUT": "10m"}},
		{"timeout unparsable", map[string]string{"OPENROUTER_EXPORTER_TIMEOUT": "later"}},
		{"stale after below timeout", map[string]string{
			"OPENROUTER_EXPORTER_TIMEOUT":     "30s",
			"OPENROUTER_EXPORTER_STALE_AFTER": "5s",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{}
			for k, v := range base {
				env[k] = v
			}
			for k, v := range tc.extra {
				env[k] = v
			}
			if _, err := LoadConfig(envMap(env)); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

func TestLoadConfigAcceptableOverrides(t *testing.T) {
	cfg, err := LoadConfig(envMap(map[string]string{
		"OPENROUTER_MANAGEMENT_KEY":       testKey,
		"OPENROUTER_EXPORTER_ADDR":        "127.0.0.1:9999",
		"OPENROUTER_EXPORTER_INTERVAL":    "15m",
		"OPENROUTER_EXPORTER_TIMEOUT":     "5s",
		"OPENROUTER_EXPORTER_STALE_AFTER": "45m",
		"OPENROUTER_API_URL":              "https://example.invalid/api/v1/credits",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Addr != "127.0.0.1:9999" || cfg.Interval != 15*time.Minute || cfg.Timeout != 5*time.Second || cfg.StaleAfter != 45*time.Minute {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
}
