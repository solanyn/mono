package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/solanyn/mono/scrib/audio"
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

	statusCmd := &cobra.Command{
		Use:   "status <uuid>",
		Short: "Show meeting status and segments",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cfg, args[0])
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List recent meetings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cfg)
		},
	}

	showCmd := &cobra.Command{
		Use:   "show <uuid>",
		Short: "Show full meeting transcript",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow(cfg, args[0])
		},
	}

	rootCmd.AddCommand(recordCmd, standupCmd, standup3Cmd, statusCmd, listCmd, showCmd)
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

func makeUploadFn(cfg *config.Config) tui.UploadFunc {
	if cfg.ServerURL == "" {
		return nil
	}
	return func(monoSamples []int16, opts tui.Options) (string, error) {
		monoPath, err := audio.WriteWAVTemp(monoSamples, opts.SampleRate, 1)
		if err != nil {
			return "", fmt.Errorf("mono wav: %w", err)
		}
		defer os.Remove(monoPath)

		dur := time.Duration(len(monoSamples)/opts.SampleRate) * time.Second
		uuid, err := uploadMeeting(cfg.ServerURL, opts.Name, dur, opts.Template)
		if err != nil {
			return "", fmt.Errorf("create meeting: %w", err)
		}

		if err := uploadAudio(cfg.ServerURL, uuid, monoPath); err != nil {
			return "", fmt.Errorf("upload audio: %w", err)
		}

		if err := requestProcess(cfg.ServerURL, uuid); err != nil {
			return "", fmt.Errorf("trigger processing: %w", err)
		}

		return uuid, nil
	}
}

func uploadMeeting(serverURL, name string, duration time.Duration, template string) (string, error) {
	body := map[string]any{
		"name":        name,
		"recorded_at": time.Now().Format(time.RFC3339),
		"duration_s":  duration.Seconds(),
		"template":    template,
	}
	data, _ := json.Marshal(body)

	resp, err := http.Post(serverURL+"/v1/meetings", "application/json", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("server returned %d: %s", resp.StatusCode, b)
	}

	var result struct {
		UUID string `json:"uuid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.UUID, nil
}

func uploadAudio(serverURL, uuid, audioPath string) error {
	f, err := os.Open(audioPath)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, _ := f.Stat()
	req, err := http.NewRequest("POST", serverURL+"/v1/audio/"+uuid, f)
	if err != nil {
		return err
	}
	req.ContentLength = stat.Size()
	req.Header.Set("Content-Type", "audio/wav")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed (%d): %s", resp.StatusCode, b)
	}
	return nil
}

func requestProcess(serverURL, uuid string) error {
	resp, err := http.Post(serverURL+"/v1/process/"+uuid, "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("process request failed (%d): %s", resp.StatusCode, b)
	}
	return nil
}

func runStatus(cfg *config.Config, uuid string) error {
	if cfg.ServerURL == "" {
		return fmt.Errorf("server_url not configured")
	}

	meeting, err := fetchMeeting(cfg.ServerURL, uuid)
	if err != nil {
		return err
	}

	name, _ := meeting["name"].(string)
	status, _ := meeting["status"].(string)
	durS, _ := meeting["duration_s"].(float64)
	dur := time.Duration(durS * float64(time.Second))

	fmt.Printf("%s  %s  %s\n", name, dur.Round(time.Second), status)

	segments, _ := meeting["segments"].([]any)
	if len(segments) > 0 {
		fmt.Printf("\n%d segments:\n", len(segments))
		for _, raw := range segments {
			seg, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			speaker, _ := seg["speaker"].(string)
			startS, _ := seg["start"].(float64)
			text, _ := seg["text"].(string)
			mins := int(startS) / 60
			secs := int(startS) % 60
			fmt.Printf("  [%02d:%02d] %s: %s\n", mins, secs, speaker, text)
		}
	}

	return nil
}

func runList(cfg *config.Config) error {
	if cfg.ServerURL == "" {
		return fmt.Errorf("server_url not configured")
	}

	resp, err := http.Get(cfg.ServerURL + "/v1/meetings")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, b)
	}

	var meetings []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&meetings); err != nil {
		return err
	}

	if len(meetings) == 0 {
		fmt.Println("No meetings.")
		return nil
	}

	for _, m := range meetings {
		uuid, _ := m["uuid"].(string)
		name, _ := m["name"].(string)
		status, _ := m["status"].(string)
		durS, _ := m["duration_s"].(float64)
		dur := time.Duration(durS * float64(time.Second))
		fmt.Printf("  %-36s  %-20s  %s  %s\n", uuid, name, dur.Round(time.Second), status)
	}
	return nil
}

func runShow(cfg *config.Config, uuid string) error {
	if cfg.ServerURL == "" {
		return fmt.Errorf("server_url not configured")
	}

	meeting, err := fetchMeeting(cfg.ServerURL, uuid)
	if err != nil {
		return err
	}

	name, _ := meeting["name"].(string)
	status, _ := meeting["status"].(string)
	durS, _ := meeting["duration_s"].(float64)
	dur := time.Duration(durS * float64(time.Second))

	fmt.Printf("# %s\n", name)
	fmt.Printf("Duration: %s | Status: %s\n\n", dur.Round(time.Second), status)

	summary, _ := meeting["summary"].(string)
	if summary != "" {
		fmt.Println(summary)
		fmt.Println()
	}

	segments, _ := meeting["segments"].([]any)
	if len(segments) > 0 {
		fmt.Println("## Transcript")
		for _, raw := range segments {
			seg, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			speaker, _ := seg["speaker"].(string)
			startS, _ := seg["start"].(float64)
			text, _ := seg["text"].(string)
			mins := int(startS) / 60
			secs := int(startS) % 60
			fmt.Printf("[%02d:%02d] %s: %s\n", mins, secs, speaker, text)
		}
	}

	return nil
}

func fetchMeeting(serverURL, uuid string) (map[string]any, error) {
	resp, err := http.Get(serverURL + "/v1/meetings/" + uuid)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, b)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
