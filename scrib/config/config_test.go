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

func TestBackwardCompatOutputDir(t *testing.T) {
	cfg := defaults()
	cfg.OutputDir = "~/old-meetings"
	cfg.Output.Dir = "~/meetings"

	if cfg.OutputDir != "" && cfg.Output.Dir == "~/meetings" {
		cfg.Output.Dir = cfg.OutputDir
	}

	if cfg.Output.Dir != "~/old-meetings" {
		t.Errorf("Output.Dir = %q, want %q", cfg.Output.Dir, "~/old-meetings")
	}
}

func TestBackwardCompatObsidianVault(t *testing.T) {
	cfg := defaults()
	cfg.ObsidianVault = "~/vault"

	if cfg.ObsidianVault != "" && cfg.Output.ObsidianVault == "" {
		cfg.Output.ObsidianVault = cfg.ObsidianVault
	}

	if cfg.Output.ObsidianVault != "~/vault" {
		t.Errorf("Output.ObsidianVault = %q, want %q", cfg.Output.ObsidianVault, "~/vault")
	}
}
