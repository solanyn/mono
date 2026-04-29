package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/solanyn/mono/scrib/audio"
	"github.com/solanyn/mono/scrib/client"
	"github.com/solanyn/mono/scrib/config"
	"github.com/solanyn/mono/scrib/tui"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	cfg := config.Load()

	var template string

	rootCmd := &cobra.Command{
		Use:     "scrib [name]",
		Short:   "Record meeting audio",
		Version: version,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecord(cfg, args, template)
		},
	}

	rootCmd.Flags().StringVarP(&template, "template", "t", "", "Summary template (standup, 1on1, planning)")

	recordCmd := &cobra.Command{
		Use:   "record [name]",
		Short: "Record system audio input and output",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecord(cfg, args, template)
		},
	}
	recordCmd.Flags().StringVarP(&template, "template", "t", "", "Summary template (standup, 1on1, planning)")

	standupCmd := &cobra.Command{
		Use:   "standup [name]",
		Short: "Record with standup template",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecord(cfg, args, "standup")
		},
	}

	standup3Cmd := &cobra.Command{
		Use:   "standup3 [name]",
		Short: "Record with standup3 template",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecord(cfg, args, "standup3")
		},
	}

	uploadCmd := &cobra.Command{
		Use:   "upload <wav-path>",
		Short: "Upload a previously recorded WAV (resume after failed upload)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpload(cfg, args[0], template)
		},
	}
	uploadCmd.Flags().StringVarP(&template, "template", "t", "", "Summary template")

	rootCmd.AddCommand(recordCmd, standupCmd, standup3Cmd, uploadCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runRecord(cfg *config.Config, args []string, template string) error {
	outDir := cfg.ExpandedOutputDir()
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	name := time.Now().Format("2006-01-02-150405")
	if len(args) > 0 && args[0] != "" {
		name = time.Now().Format("2006-01-02") + "-" + args[0]
	}
	outPath := filepath.Join(outDir, name+".wav")

	opts := tui.Options{
		Name:       name,
		OutputPath: outPath,
		SampleRate: cfg.SampleRate,
		ServerURL:  cfg.ServerURL,
		Template:   template,
	}

	return tui.Run(opts, makeUploadFn(cfg))
}

// runUpload re-uploads an existing WAV file (e.g. after a failed first attempt).
// Uses the mono WAV on disk directly — no TUI, just stdout progress.
func runUpload(cfg *config.Config, wavPath, template string) error {
	if cfg.ServerURL == "" {
		return fmt.Errorf("server_url not configured")
	}
	abs, err := filepath.Abs(wavPath)
	if err != nil {
		return err
	}
	name := strip(filepath.Base(abs), ".wav")

	samples, err := audio.ReadWAVMono(abs)
	if err != nil {
		// If stereo, downmix to a new temp mono WAV first.
		return fmt.Errorf("read wav: %w (tip: mono WAV expected; pass the upload copy, not the raw stereo)", err)
	}
	dur := time.Duration(len(samples)/cfg.SampleRate) * time.Second

	ctx := context.Background()
	res, err := client.Upload(ctx, client.UploadInput{
		ServerURL: cfg.ServerURL,
		Name:      name,
		Template:  template,
		Duration:  dur,
		WAVPath:   abs,
	}, func(p client.Progress) {
		if p.Err != nil {
			fmt.Fprintf(os.Stderr, "  %s attempt %d failed: %v\n", p.Stage, p.Attempt, p.Err)
		} else {
			fmt.Fprintf(os.Stderr, "  %s ok\n", p.Stage)
		}
	})
	if err != nil {
		return err
	}
	fmt.Printf("Uploaded: %s (%s)\n", name, res.UUID)
	return nil
}

func strip(s, suffix string) string {
	if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
		return s[:len(s)-len(suffix)]
	}
	return s
}

func makeUploadFn(cfg *config.Config) tui.UploadFunc {
	if cfg.ServerURL == "" {
		return nil
	}
	return func(ctx context.Context, monoSamples []int16, opts tui.Options, progress func(tui.ProgressEvent)) (string, error) {
		monoPath, err := audio.WriteWAVTemp(monoSamples, opts.SampleRate, 1)
		if err != nil {
			return "", fmt.Errorf("mono wav: %w", err)
		}
		defer os.Remove(monoPath)

		dur := time.Duration(len(monoSamples)/opts.SampleRate) * time.Second
		res, err := client.Upload(ctx, client.UploadInput{
			ServerURL: opts.ServerURL,
			Name:      opts.Name,
			Template:  opts.Template,
			Duration:  dur,
			WAVPath:   monoPath,
		}, func(p client.Progress) {
			progress(tui.ProgressEvent{Stage: p.Stage, Attempt: p.Attempt, Err: p.Err})
		})
		if err != nil {
			return "", err
		}
		return res.UUID, nil
	}
}
