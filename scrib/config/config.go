package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml"
)

type CalendarConfig struct {
	URL      string `toml:"url"`
	User     string `toml:"user"`
	TokenCmd string `toml:"token_cmd"`
}

type OutputConfig struct {
	Dir           string `toml:"dir"`
	ObsidianVault string `toml:"obsidian_vault"`
}

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
	SampleRate    int             `toml:"sample_rate"`
	Format        string          `toml:"format"`
	InputDevice   string          `toml:"input_device"`
	OutputDir     string          `toml:"output_dir"`
	ObsidianVault string          `toml:"obsidian_vault"`
	Output        OutputConfig    `toml:"output"`
	Calendar      CalendarConfig  `toml:"calendar"`
	Scribe        ScribeConfig    `toml:"scribe"`
	Summarise     SummariseConfig `toml:"summarise"`
	Sync          SyncConfig      `toml:"sync"`
}

func defaults() *Config {
	return &Config{
		GatewayURL: "https://gateway.goyangi.io/v1/opus",
		AudioURL:   "http://127.0.0.1:8000",
		STTModel:   "mlx-community/parakeet-tdt-0.6b-v3",
		SampleRate: 16000,
		Format:     "wav",
		Output: OutputConfig{
			Dir: "~/meetings",
		},
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

	// Env overrides
	if v := os.Getenv("SCRIB_GATEWAY_URL"); v != "" {
		cfg.GatewayURL = v
	}
	if v := os.Getenv("SCRIB_AUDIO_URL"); v != "" {
		cfg.AudioURL = v
	}
	if v := os.Getenv("SCRIB_OUTPUT_DIR"); v != "" {
		cfg.Output.Dir = v
	}
	if v := os.Getenv("SCRIB_OBSIDIAN_VAULT"); v != "" {
		cfg.Output.ObsidianVault = v
	}
	if v := os.Getenv("SCRIB_CALENDAR_URL"); v != "" {
		cfg.Calendar.URL = v
	}
	if v := os.Getenv("SCRIB_CALENDAR_USER"); v != "" {
		cfg.Calendar.User = v
	}

	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 16000
	}
	return cfg
}

// CalendarToken returns the calendar auth token.
// Precedence: SCRIB_CALENDAR_TOKEN env > token_cmd config > empty.
func (c *Config) CalendarToken() string {
	if v := os.Getenv("SCRIB_CALENDAR_TOKEN"); v != "" {
		return v
	}
	if c.Calendar.TokenCmd != "" {
		out, err := exec.Command("sh", "-c", c.Calendar.TokenCmd).Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

// GatewayAPIKey returns the gateway API key from env.
func (c *Config) GatewayAPIKey() string {
	return os.Getenv("SCRIB_GATEWAY_API_KEY")
}

func (c *Config) ExpandedOutputDir() string {
	dir := c.Output.Dir
	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	}
	return dir
}
