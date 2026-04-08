package main

import (
	"fmt"
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
	"github.com/solanyn/mono/scrib/tui"
	"github.com/spf13/cobra"
)

func main() {
	cfg := config.Load()

	rootCmd := &cobra.Command{
		Use:   "scrib [name]",
		Short: "Meeting audio capture & annotation",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if template != "" {
				cfg.Summarise.Template = template
			}
			return runTUI(cfg, args)
		},
	}

	rootCmd.Flags().StringVarP(&template, "template", "t", "", "Summary template (standup, 1on1, planning)")

	var annotateAfter bool
	var template string

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

	rootCmd.AddCommand(recordCmd, annotateCmd, historyCmd, searchCmd, speakersCmd, showCmd)
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

	// Store in database
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
	c := client.New(cfg.AudioURL, cfg.GatewayURL)

	fmt.Printf("Annotating %s...\n", audioPath)
	fmt.Println("  → Running VAD + STT (concurrent)...")

	result, err := c.Annotate(audioPath, 0.5, cfg.Summarise.Template)
	if err != nil {
		return fmt.Errorf("annotate: %w", err)
	}

	// Store in database
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

		// Store segments
		for _, seg := range result.Segments {
			db.InsertSegment(&store.Segment{
				MeetingID:    meetingID,
				SpeakerLabel: seg.Speaker,
				StartS:       seg.Start,
				EndS:         seg.End,
				Text:         seg.Text,
			})
		}

		// Store summary
		db.InsertSummary(&store.Summary{
			MeetingID: meetingID,
			Template:  cfg.Summarise.Template,
			Content:   result.Summary,
		})
	}

	// Write markdown output
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

	fmt.Printf("  → %d speakers, %s duration\n", result.RawVAD.NumSpeakers, result.Duration)
	fmt.Printf("  → Saved to %s\n", outPath)
	if meetingID > 0 {
		fmt.Printf("  → Stored in db (meeting #%d)\n", meetingID)
	}

	if cfg.ObsidianVault != "" {
		vaultDir := expandPath(cfg.ObsidianVault)
		os.MkdirAll(vaultDir, 0755)
		vaultPath := filepath.Join(vaultDir, filepath.Base(outPath))
		if err := os.WriteFile(vaultPath, []byte(sb.String()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: couldn't write to vault: %v\n", err)
		} else {
			fmt.Printf("  → Synced to %s\n", vaultPath)
		}
	}

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

	// Show latest summary
	summaries, _ := db.GetSummaries(id)
	if len(summaries) > 0 {
		fmt.Println(summaries[0].Content)
		fmt.Println()
	}

	// Show transcript
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

	c := client.New(cfg.AudioURL, cfg.GatewayURL)

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
		Client:     c,
		Template:   cfg.Summarise.Template,
		DB:         db,
	})
}
