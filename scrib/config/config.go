package config

import (
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

type Config struct {
	ServerURL   string `toml:"server_url"`
	SampleRate  int    `toml:"sample_rate"`
	OutputDir   string `toml:"output_dir"`
	InputDevice string `toml:"input_device"`
}

func defaults() *Config {
	return &Config{
		ServerURL:  "https://scrib.goyangi.io",
		SampleRate: 16000,
		OutputDir:  "~/meetings",
	}
}

func Load() *Config {
	cfg := defaults()

	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, ".config", "scrib", "config.toml")

	if data, err := os.ReadFile(cfgPath); err == nil {
		toml.Unmarshal(data, cfg)
	}

	if v := os.Getenv("SCRIB_SERVER_URL"); v != "" {
		cfg.ServerURL = v
	}
	if v := os.Getenv("SCRIB_OUTPUT_DIR"); v != "" {
		cfg.OutputDir = v
	}
	if v := os.Getenv("SCRIB_INPUT_DEVICE"); v != "" {
		cfg.InputDevice = v
	}

	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 16000
	}
	return cfg
}

func (c *Config) ExpandedOutputDir() string {
	dir := c.OutputDir
	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	}
	return dir
}
