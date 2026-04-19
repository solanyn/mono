package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/solanyn/mono/scrib/audio"
	"github.com/solanyn/mono/scrib/client"
	"github.com/solanyn/mono/scrib/config"
	"github.com/solanyn/mono/scrib/store"
	"github.com/solanyn/mono/scrib/sync"
	"github.com/solanyn/mono/scrib/tui"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	cfg := config.Load()

	var template string

	rootCmd := &cobra.Command{
		Use:     "scrib [name]",
		Short:   "Meeting audio capture & annotation",
		Version: version,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if template != "" {
				cfg.Summarise.Template = template
			}
			return runTUI(cfg, args)
		},
	}

	rootCmd.Flags().StringVarP(&template, "template", "t", "", "Summary template (standup, 1on1, planning)")

	var annotateAfter bool

	recordCmd := &cobra.Command{
		Use:   "record [name]",
		Short: "Record system audio input and output",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if template != "" {
				cfg.Summarise.Template = template
			}
			return runRecord(cfg, args, annotateAfter)
		},
	}
	recordCmd.Flags().BoolVar(&annotateAfter, "annotate", false, "Run annotation pipeline after recording stops")
	recordCmd.Flags().StringVarP(&template, "template", "t", "", "Summary template (standup, 1on1, planning)")

	annotateCmd := &cobra.Command{
		Use:   "annotate <file>",
		Short: "Diarise and summarise a recording",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if template != "" {
				cfg.Summarise.Template = template
			}
			return runAnnotate(cfg, args[0])
		},
	}
	annotateCmd.Flags().StringVarP(&template, "template", "t", "", "Summary template (standup, 1on1, planning)")

	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "List meetings from database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistory(cfg)
		},
	}

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across all transcripts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cfg, args[0])
		},
	}

	speakersCmd := &cobra.Command{
		Use:   "speakers",
		Short: "List known speakers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSpeakers(cfg)
		},
	}

	speakersAddCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a known speaker",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSpeakersAdd(cfg, args[0])
		},
	}
	speakersCmd.AddCommand(speakersAddCmd)

	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show meeting details and transcript",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow(cfg, args[0])
		},
	}

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync meetings to/from scrib server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(cfg)
		},
	}

	resummarizeCmd := &cobra.Command{
		Use:   "resummarize <meeting_id>",
		Short: "Retry summary for a meeting that has segments but no summary",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if template != "" {
				cfg.Summarise.Template = template
			}
			return runResummarize(cfg, args[0])
		},
	}
	resummarizeCmd.Flags().StringVarP(&template, "template", "t", "", "Summary template (standup, 1on1, planning)")

	transcribeCmd := &cobra.Command{
		Use:   "transcribe <uuid>",
		Short: "Trigger server-side transcription and print results",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTranscribe(cfg, args[0])
		},
	}

	rootCmd.AddCommand(recordCmd, annotateCmd, historyCmd, searchCmd, speakersCmd, showCmd, syncCmd, resummarizeCmd, transcribeCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func openDB() (*store.DB, error) {
	return store.Open(store.DefaultPath())
}

func runRecord(cfg *config.Config, args []string, annotateAfter bool) error {
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

	db, err := openDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: couldn't open db: %v\n", err)
	} else {
		defer db.Close()
		db.InsertMeeting(&store.Meeting{
			Name:       name,
			RecordedAt: time.Now(),
			DurationS:  dur.Seconds(),
			Template:   cfg.Summarise.Template,
			AudioPath:  outPath,
		})
	}

	if annotateAfter {
		return runAnnotate(cfg, outPath)
	}
	return nil
}

func runAnnotate(cfg *config.Config, audioPath string) error {
	c := client.New(cfg.AudioURL, cfg.GatewayURL, cfg.APIKey, cfg.STTModel, cfg.Summarise.Model)

	fmt.Printf("Annotating %s...\n", audioPath)
	fmt.Println("  → Running VAD + STT (concurrent)...")

	result, err := c.Annotate(context.Background(), audioPath, 0.5, cfg.Summarise.Template)
	if err != nil {
		return fmt.Errorf("annotate: %w", err)
	}

	db, dbErr := openDB()
	var meetingID int64
	if dbErr == nil {
		defer db.Close()
		name := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
		meetingID, _ = db.InsertMeeting(&store.Meeting{
			Name:        name,
			RecordedAt:  time.Now(),
			DurationS:   result.RawVAD.DurationSeconds,
			Template:    cfg.Summarise.Template,
			AudioPath:   audioPath,
			NumSpeakers: result.RawVAD.NumSpeakers,
		})

		for _, seg := range result.Segments {
			db.InsertSegment(&store.Segment{
				MeetingID:    meetingID,
				SpeakerLabel: seg.Speaker,
				StartS:       seg.Start,
				EndS:         seg.End,
				Text:         seg.Text,
			})
		}

		if result.SummaryErr != nil {
			fmt.Fprintf(os.Stderr, "warning: summary failed (segments saved): %v\n", result.SummaryErr)
			fmt.Printf("  → Stored segments in db (meeting #%d) — run 'scrib resummarize %d' to retry\n", meetingID, meetingID)
		} else {
			db.InsertSummary(&store.Summary{
				MeetingID: meetingID,
				Template:  cfg.Summarise.Template,
				Content:   result.Summary,
			})
		}
	}

	fmt.Printf("  → %d speakers, %s duration\n", result.RawVAD.NumSpeakers, result.Duration)

	if result.Summary != "" {
		outPath := strings.TrimSuffix(audioPath, filepath.Ext(audioPath)) + ".md"
		name := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
		now := time.Now().Format("2006-01-02 15:04")

		var sb strings.Builder
		fmt.Fprintf(&sb, "# %s — %s\n\n", name, now)
		sb.WriteString(result.Summary)
		sb.WriteString("\n")

		if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		fmt.Printf("  → Saved to %s\n", outPath)

		if cfg.Output.ObsidianVault != "" {
			vaultDir := expandPath(cfg.Output.ObsidianVault)
			os.MkdirAll(vaultDir, 0755)
			vaultPath := filepath.Join(vaultDir, filepath.Base(outPath))
			if err := os.WriteFile(vaultPath, []byte(sb.String()), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "warning: couldn't write to vault: %v\n", err)
			} else {
				fmt.Printf("  → Synced to %s\n", vaultPath)
			}
		}
	}

	if meetingID > 0 {
		fmt.Printf("  → Stored in db (meeting #%d)\n", meetingID)
	}

	return nil
}

func runResummarize(cfg *config.Config, idStr string) error {
	var id int64
	fmt.Sscanf(idStr, "%d", &id)
	if id <= 0 {
		return fmt.Errorf("invalid meeting ID: %s", idStr)
	}

	db, err := openDB()
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	meeting, err := db.GetMeeting(id)
	if err != nil {
		return fmt.Errorf("get meeting: %w", err)
	}

	segments, err := db.GetSegments(id)
	if err != nil {
		return fmt.Errorf("get segments: %w", err)
	}
	if len(segments) == 0 {
		return fmt.Errorf("meeting #%d has no segments", id)
	}

	transcript := store.FormatTranscript(segments)

	tmpl := cfg.Summarise.Template
	if meeting.Template != "" {
		tmpl = meeting.Template
	}

	c := client.New(cfg.AudioURL, cfg.GatewayURL, cfg.APIKey, cfg.STTModel, cfg.Summarise.Model)
	fmt.Printf("Re-summarizing meeting #%d (%s)...\n", id, meeting.Name)

	summary, err := c.Summarize(context.Background(), transcript, tmpl)
	if err != nil {
		return fmt.Errorf("summarize: %w", err)
	}

	if _, err := db.InsertSummary(&store.Summary{
		MeetingID: id,
		Template:  tmpl,
		Content:   summary,
	}); err != nil {
		return fmt.Errorf("insert summary: %w", err)
	}

	fmt.Println(summary)
	fmt.Printf("  → Summary stored for meeting #%d\n", id)
	return nil
}

func runHistory(cfg *config.Config) error {
	db, err := openDB()
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	meetings, err := db.ListMeetings(20)
	if err != nil {
		return fmt.Errorf("list meetings: %w", err)
	}

	if len(meetings) == 0 {
		fmt.Println("No meetings yet.")
		return nil
	}

	for _, m := range meetings {
		dur := time.Duration(m.DurationS * float64(time.Second))
		fmt.Printf("  #%-4d  %s  %-20s  %s  %d speakers\n",
			m.ID,
			m.RecordedAt.Format("2006-01-02 15:04"),
			m.Name,
			dur.Round(time.Second),
			m.NumSpeakers,
		)
	}
	return nil
}

func runSearch(cfg *config.Config, query string) error {
	db, err := openDB()
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	results, err := db.Search(query, 20)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		fmt.Printf("No results for %q\n", query)
		return nil
	}

	for _, r := range results {
		mins := int(r.StartS) / 60
		secs := int(r.StartS) % 60
		fmt.Printf("  [%s %s] %s (%d:%02d): %s\n",
			r.RecordedAt.Format("2006-01-02"),
			r.MeetingName,
			r.SpeakerLabel,
			mins, secs,
			r.Text,
		)
	}
	return nil
}

func runSpeakers(cfg *config.Config) error {
	db, err := openDB()
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	speakers, err := db.ListSpeakers()
	if err != nil {
		return fmt.Errorf("list speakers: %w", err)
	}

	if len(speakers) == 0 {
		fmt.Println("No known speakers yet. Use 'scrib speakers add <name>' to add one.")
		return nil
	}

	for _, s := range speakers {
		hasEmbed := "no voiceprint"
		if len(s.Embedding) > 0 {
			hasEmbed = "has voiceprint"
		}
		fmt.Printf("  #%-4d  %-20s  %s\n", s.ID, s.Name, hasEmbed)
	}
	return nil
}

func runSpeakersAdd(cfg *config.Config, name string) error {
	db, err := openDB()
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	id, err := db.InsertSpeaker(name, nil)
	if err != nil {
		return fmt.Errorf("add speaker: %w", err)
	}

	fmt.Printf("Added speaker %q (#%d)\n", name, id)
	return nil
}

func runShow(cfg *config.Config, idStr string) error {
	db, err := openDB()
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	var id int64
	fmt.Sscanf(idStr, "%d", &id)

	meeting, err := db.GetMeeting(id)
	if err != nil {
		return fmt.Errorf("get meeting: %w", err)
	}

	dur := time.Duration(meeting.DurationS * float64(time.Second))
	fmt.Printf("# %s — %s\n", meeting.Name, meeting.RecordedAt.Format("2006-01-02 15:04"))
	fmt.Printf("Duration: %s | Speakers: %d | Template: %s\n", dur.Round(time.Second), meeting.NumSpeakers, meeting.Template)
	fmt.Printf("Audio: %s\n\n", meeting.AudioPath)

	summaries, _ := db.GetSummaries(id)
	if len(summaries) > 0 {
		fmt.Println(summaries[0].Content)
		fmt.Println()
	}

	segments, _ := db.GetSegments(id)
	if len(segments) > 0 {
		fmt.Println("## Transcript")
		fmt.Print(store.FormatTranscript(segments))
	}

	return nil
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func runTUI(cfg *config.Config, args []string) error {
	outDir := cfg.ExpandedOutputDir()
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	name := time.Now().Format("2006-01-02-150405")
	if len(args) > 0 && args[0] != "" {
		name = time.Now().Format("2006-01-02") + "-" + args[0]
	}
	outPath := filepath.Join(outDir, name+".wav")

	c := client.New(cfg.AudioURL, cfg.GatewayURL, cfg.APIKey, cfg.STTModel, cfg.Summarise.Model)

	db, _ := openDB()
	if db != nil {
		defer db.Close()
	}

	return tui.Run(tui.Options{
		Name:       name,
		OutputPath: outPath,
		SampleRate: cfg.SampleRate,
		AudioURL:   cfg.AudioURL,
		GatewayURL: cfg.GatewayURL,
		ServerURL:  cfg.Sync.ServerURL,
		Client:     c,
		Template:   cfg.Summarise.Template,
		DB:         db,
	})
}

func runSync(cfg *config.Config) error {
	if cfg.Sync.ServerURL == "" {
		return fmt.Errorf("sync.server_url not configured in ~/.config/scrib/config.toml")
	}

	db, err := openDB()
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	clientID := cfg.Sync.ClientID
	if clientID == "" {
		hostname, _ := os.Hostname()
		clientID = hostname
	}

	sc := sync.NewClient(cfg.Sync.ServerURL, clientID, db)

	fmt.Println("Pushing unsynced meetings...")
	pushResult, err := sc.Push()
	if err != nil {
		return fmt.Errorf("push: %w", err)
	}
	if pushResult.Synced > 0 {
		fmt.Printf("  → Pushed %d meetings\n", pushResult.Synced)
	} else {
		fmt.Println("  → Nothing to push")
	}

	fmt.Println("Pulling from server...")
	pulled, err := sc.Pull()
	if err != nil {
		return fmt.Errorf("pull: %w", err)
	}
	if pulled > 0 {
		fmt.Printf("  → Pulled %d meetings\n", pulled)
	} else {
		fmt.Println("  → Nothing new")
	}

	return nil
}

func runTranscribe(cfg *config.Config, uuid string) error {
	serverURL := cfg.Sync.ServerURL
	if serverURL == "" {
		return fmt.Errorf("sync.server_url not configured in ~/.config/scrib/config.toml")
	}

	meeting, err := fetchMeeting(serverURL, uuid)
	if err != nil {
		return fmt.Errorf("fetch meeting: %w", err)
	}

	status, _ := meeting["status"].(string)

	if status != "done" {
		if status != "processing" {
			resp, err := http.Post(serverURL+"/v1/process/"+uuid, "", nil)
			if err != nil {
				return fmt.Errorf("trigger processing: %w", err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusAccepted {
				return fmt.Errorf("process request failed (%d)", resp.StatusCode)
			}
			fmt.Println("Processing started...")
		} else {
			fmt.Println("Already processing...")
		}

		deadline := time.After(5 * time.Minute)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-deadline:
				return fmt.Errorf("timed out waiting for processing (5m)")
			case <-ticker.C:
				meeting, err = fetchMeeting(serverURL, uuid)
				if err != nil {
					return fmt.Errorf("poll meeting: %w", err)
				}
				status, _ = meeting["status"].(string)
				if status == "done" {
					goto done
				}
				if status == "error" {
					errMsg, _ := meeting["error"].(string)
					return fmt.Errorf("processing failed: %s", errMsg)
				}
			}
		}
	}
done:

	segments, _ := meeting["segments"].([]any)
	if len(segments) == 0 {
		fmt.Println("No segments found.")
		return nil
	}

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
