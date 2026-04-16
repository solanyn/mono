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
