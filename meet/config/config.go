package config

import (
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml"
)

type Config struct {
	GatewayURL string `toml:"gateway_url"`
	OutputDir  string `toml:"output_dir"`
	SampleRate int    `toml:"sample_rate"`
	Format     string `toml:"format"`
}

func defaults() *Config {
	return &Config{
		GatewayURL: "https://gateway.goyangi.io",
		OutputDir:  "~/meetings",
		SampleRate: 16000,
		Format:     "wav",
	}
}

func Load() *Config {
	cfg := defaults()

	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, ".config", "meet", "config.toml")

	if data, err := os.ReadFile(cfgPath); err == nil {
		toml.Unmarshal(data, cfg)
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
