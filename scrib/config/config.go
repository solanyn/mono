package config

import (
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml"
)

type ScribeConfig struct {
	NoiseRemoval bool `toml:"noise_removal"`
	AutoScroll   bool `toml:"auto_scroll"`
	NotesWidth   int  `toml:"notes_width"`
}

type SummariseConfig struct {
	Model    string `toml:"model"`
	Template string `toml:"template"`
}

type SyncConfig struct {
	ServerURL string `toml:"server_url"`
	ClientID  string `toml:"client_id"`
}

type Config struct {
	GatewayURL    string          `toml:"gateway_url"`
	AudioURL      string          `toml:"audio_url"`
	APIKey        string          `toml:"api_key"`
	STTModel      string          `toml:"stt_model"`
	OutputDir     string          `toml:"output_dir"`
	ObsidianVault string          `toml:"obsidian_vault"`
	SampleRate    int             `toml:"sample_rate"`
	Format        string          `toml:"format"`
	Scribe        ScribeConfig    `toml:"scribe"`
	Summarise     SummariseConfig `toml:"summarise"`
	Sync          SyncConfig      `toml:"sync"`
}

func defaults() *Config {
	return &Config{
		GatewayURL:   "https://gateway.goyangi.io",
		AudioURL:     "http://127.0.0.1:8000",
		STTModel:     "mlx-community/parakeet-tdt-0.6b-v3",
		OutputDir:    "~/meetings",
		SampleRate:   16000,
		Format:       "wav",
		Scribe: ScribeConfig{
			NoiseRemoval: true,
			AutoScroll:   true,
			NotesWidth:   30,
		},
		Summarise: SummariseConfig{
			Model:    "auto",
			Template: "standup",
		},
	}
}

func Load() *Config {
	cfg := defaults()

	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, ".config", "scrib", "config.toml")

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
