package config

import (
	"os"
	"testing"
)

func TestDefaultGatewayURL(t *testing.T) {
	cfg := defaults()
	want := "https://gateway.goyangi.io/v1/opus"
	if cfg.GatewayURL != want {
		t.Errorf("default GatewayURL = %q, want %q", cfg.GatewayURL, want)
	}
}

func TestDefaultAudioURL(t *testing.T) {
	cfg := defaults()
	want := "http://127.0.0.1:8000"
	if cfg.AudioURL != want {
		t.Errorf("default AudioURL = %q, want %q", cfg.AudioURL, want)
	}
}

func TestDefaultInputDeviceEmpty(t *testing.T) {
	cfg := defaults()
	if cfg.InputDevice != "" {
		t.Errorf("default InputDevice = %q, want empty", cfg.InputDevice)
	}
}

func TestInputDeviceEnvOverride(t *testing.T) {
	os.Setenv("SCRIB_INPUT_DEVICE", "Blue Yeti")
	defer os.Unsetenv("SCRIB_INPUT_DEVICE")

	cfg := Load()
	if cfg.InputDevice != "Blue Yeti" {
		t.Errorf("InputDevice = %q, want %q", cfg.InputDevice, "Blue Yeti")
	}
}

func TestDefaultSampleRate(t *testing.T) {
	cfg := defaults()
	if cfg.SampleRate != 16000 {
		t.Errorf("default SampleRate = %d, want 16000", cfg.SampleRate)
	}
}

func TestMigrateLegacyOutputDir(t *testing.T) {
	cfg := defaults()
	cfg.OutputDir = "~/old-meetings"
	cfg.Output.Dir = ""

	migrateDeprecatedFields(cfg)

	if cfg.Output.Dir != "~/old-meetings" {
		t.Errorf("Output.Dir = %q, want %q", cfg.Output.Dir, "~/old-meetings")
	}
	if cfg.OutputDir != "" {
		t.Errorf("OutputDir should be cleared after migration, got %q", cfg.OutputDir)
	}
}

func TestMigrateLegacyObsidianVault(t *testing.T) {
	cfg := defaults()
	cfg.ObsidianVault = "~/vault"

	migrateDeprecatedFields(cfg)

	if cfg.Output.ObsidianVault != "~/vault" {
		t.Errorf("Output.ObsidianVault = %q, want %q", cfg.Output.ObsidianVault, "~/vault")
	}
	if cfg.ObsidianVault != "" {
		t.Errorf("ObsidianVault should be cleared after migration, got %q", cfg.ObsidianVault)
	}
}

func TestNestedConfigUnchanged(t *testing.T) {
	cfg := defaults()
	cfg.Output.Dir = "~/my-meetings"
	cfg.Output.ObsidianVault = "~/my-vault"

	migrateDeprecatedFields(cfg)

	if cfg.Output.Dir != "~/my-meetings" {
		t.Errorf("Output.Dir = %q, want %q", cfg.Output.Dir, "~/my-meetings")
	}
	if cfg.Output.ObsidianVault != "~/my-vault" {
		t.Errorf("Output.ObsidianVault = %q, want %q", cfg.Output.ObsidianVault, "~/my-vault")
	}
}

func TestConflictingValuesNestedWins(t *testing.T) {
	cfg := defaults()
	cfg.OutputDir = "~/old"
	cfg.Output.Dir = "~/new"

	migrateDeprecatedFields(cfg)

	if cfg.Output.Dir != "~/new" {
		t.Errorf("Output.Dir = %q, want %q (nested should win)", cfg.Output.Dir, "~/new")
	}
	if cfg.OutputDir != "" {
		t.Errorf("OutputDir should be cleared after migration, got %q", cfg.OutputDir)
	}
}
