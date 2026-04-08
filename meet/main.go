package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/solanyn/mono/meet/audio"
	"github.com/solanyn/mono/meet/config"
	"github.com/spf13/cobra"
)

func main() {
	cfg := config.Load()

	rootCmd := &cobra.Command{
		Use:   "meet",
		Short: "Meeting audio capture & annotation",
	}

	recordCmd := &cobra.Command{
		Use:   "record [name]",
		Short: "Record system audio input and output",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecord(cfg, args)
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List recordings in output directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cfg)
		},
	}

	rootCmd.AddCommand(recordCmd, listCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runRecord(cfg *config.Config, args []string) error {
	outDir := cfg.ExpandedOutputDir()
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	name := time.Now().Format("2006-01-02-150405")
	if len(args) > 0 && args[0] != "" {
		name = time.Now().Format("2006-01-02") + "-" + args[0]
	}
	outPath := filepath.Join(outDir, name+".wav")

	rec, err := audio.NewRecorder(cfg.SampleRate)
	if err != nil {
		return fmt.Errorf("init recorder: %w", err)
	}

	fmt.Printf("Recording to %s\n", outPath)
	fmt.Println("Press Ctrl+C to stop...")

	if err := rec.Start(); err != nil {
		return fmt.Errorf("start recording: %w", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("\nStopping...")
	samples := rec.Stop()

	if err := audio.WriteWAV(outPath, samples, cfg.SampleRate, 2); err != nil {
		return fmt.Errorf("write wav: %w", err)
	}

	dur := time.Duration(len(samples)/2/cfg.SampleRate) * time.Second
	fmt.Printf("Saved %s (%s)\n", outPath, dur)
	return nil
}

func runList(cfg *config.Config) error {
	outDir := cfg.ExpandedOutputDir()
	entries, err := os.ReadDir(outDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No recordings yet.")
			return nil
		}
		return err
	}

	var found bool
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".wav") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		found = true
		sizeMB := float64(info.Size()) / 1024 / 1024
		fmt.Printf("  %s  (%.1f MB)\n", e.Name(), sizeMB)
	}
	if !found {
		fmt.Println("No recordings yet.")
	}
	return nil
}
