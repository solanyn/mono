package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/solanyn/mono/line/internal/db"
	"github.com/solanyn/mono/line/internal/kafka"
)

func (s *server) handleGenerateBriefing(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	summaryKey := "sessions/" + sessionID + "/summary.json"
	summaryData, err := s.gold.GetObject(r.Context(), summaryKey)
	if err != nil {
		http.Error(w, "session summary not found — complete a session first", http.StatusNotFound)
		return
	}

	var summary struct {
		CarCode     int32  `json:"car_code"`
		TrackName   string `json:"track_name"`
		LapCount    int    `json:"lap_count"`
		Consistency struct {
			ConsistencyScore float64 `json:"consistency_score"`
			LapTimeCV        float64 `json:"lap_time_cv"`
			BestWorstDeltaMs int     `json:"best_worst_delta_ms"`
		} `json:"consistency"`
		TyreDegradation struct {
			DegradationRate     float64 `json:"degradation_rate"`
			EstimatedLapsRemain int     `json:"estimated_laps_remaining"`
			CompoundGuess       string  `json:"compound_guess"`
			FrontRearBalance    float64 `json:"front_rear_balance"`
		} `json:"tyre_degradation"`
		FuelStrategy struct {
			ConsumptionPerLap float64 `json:"consumption_per_lap"`
			LapsRemaining     int     `json:"laps_remaining"`
			OptimalPitLap     int     `json:"optimal_pit_lap"`
		} `json:"fuel_strategy"`
		Journal struct {
			BestLapMs      int      `json:"best_lap_ms"`
			WorstLapMs     int      `json:"worst_lap_ms"`
			Highlights     []string `json:"highlights"`
			AreasToImprove []string `json:"areas_to_improve"`
			CornerNotes    []string `json:"corner_notes"`
		} `json:"journal"`
	}
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		slog.Error("parse summary for briefing", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var trackHistory []db.SessionHistory
	if s.database != nil && summary.TrackName != "" {
		session, _ := s.database.GetSession(r.Context(), sessionID)
		if session != nil && session.TrackID != nil {
			trackHistory, _ = s.database.GetTrackHistory(r.Context(), *session.TrackID, 10)
		}
	}

	prompt := buildBriefingPrompt(summary.TrackName, summary.CarCode, summary.LapCount,
		summary.Consistency.ConsistencyScore, summary.Consistency.LapTimeCV, summary.Consistency.BestWorstDeltaMs,
		summary.Journal.BestLapMs, summary.Journal.WorstLapMs,
		summary.TyreDegradation.DegradationRate, summary.TyreDegradation.CompoundGuess, summary.TyreDegradation.FrontRearBalance,
		summary.FuelStrategy.ConsumptionPerLap, summary.FuelStrategy.LapsRemaining, summary.FuelStrategy.OptimalPitLap,
		summary.Journal.Highlights, summary.Journal.AreasToImprove, summary.Journal.CornerNotes,
		trackHistory, s.cars)

	briefing, err := s.llm.GenerateBriefing(r.Context(), prompt)
	if err != nil {
		slog.Error("llm briefing generation", "err", err)
		http.Error(w, "briefing generation failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"session_id": sessionID,
		"briefing":   briefing,
	})
}

func (s *server) handleGetJournal(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if s.database == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}
	journal, err := s.database.GetJournal(r.Context(), sessionID)
	if err != nil {
		http.Error(w, "journal not found", http.StatusNotFound)
		return
	}
	writeJSON(w, journal)
}

func (s *server) handleGenerateJournal(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if s.database == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}

	summaryKey := "sessions/" + sessionID + "/summary.json"
	summaryData, err := s.gold.GetObject(r.Context(), summaryKey)
	if err != nil {
		http.Error(w, "session summary not found", http.StatusNotFound)
		return
	}

	var summary struct {
		CarCode     int32  `json:"car_code"`
		TrackName   string `json:"track_name"`
		LapCount    int    `json:"lap_count"`
		Consistency struct {
			ConsistencyScore float64 `json:"consistency_score"`
			LapTimeCV        float64 `json:"lap_time_cv"`
			BestLapIdx       int     `json:"best_lap_idx"`
			WorstLapIdx      int     `json:"worst_lap_idx"`
			BestWorstDeltaMs int     `json:"best_worst_delta_ms"`
		} `json:"consistency"`
		TyreDegradation struct {
			DegradationRate  float64 `json:"degradation_rate"`
			CompoundGuess    string  `json:"compound_guess"`
			FrontRearBalance float64 `json:"front_rear_balance"`
		} `json:"tyre_degradation"`
		Journal struct {
			BestLapMs      int      `json:"best_lap_ms"`
			WorstLapMs     int      `json:"worst_lap_ms"`
			Highlights     []string `json:"highlights"`
			AreasToImprove []string `json:"areas_to_improve"`
			CornerNotes    []string `json:"corner_notes"`
		} `json:"journal"`
	}
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	prompt := buildJournalPrompt(summary.TrackName, summary.CarCode, summary.LapCount,
		summary.Consistency.ConsistencyScore, summary.Consistency.BestWorstDeltaMs,
		summary.Consistency.BestLapIdx, summary.Consistency.WorstLapIdx,
		summary.Journal.BestLapMs, summary.Journal.WorstLapMs,
		summary.TyreDegradation.DegradationRate, summary.TyreDegradation.CompoundGuess,
		summary.Journal.Highlights, summary.Journal.AreasToImprove, summary.Journal.CornerNotes,
		s.cars)

	content, err := s.llm.GenerateJournal(r.Context(), prompt)
	if err != nil {
		slog.Error("llm journal generation", "err", err)
		http.Error(w, "journal generation failed", http.StatusInternalServerError)
		return
	}

	journal := &db.Journal{
		SessionID: sessionID,
		Content:   content,
	}
	if err := s.database.SaveJournal(r.Context(), journal); err != nil {
		slog.Error("save journal", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, journal)
}

func (s *server) runJournalWorker(ctx context.Context, consumer *kafka.Consumer) {
	slog.Info("journal worker started")
	consumer.Run(ctx, func(ctx context.Context, record *kgo.Record) error {
		var event struct {
			SessionID string `json:"session_id"`
			CarCode   int32  `json:"car_code"`
			LapCount  int    `json:"lap_count"`
		}
		if err := json.Unmarshal(record.Value, &event); err != nil {
			slog.Error("journal worker: unmarshal event", "err", err)
			return nil
		}

		if s.database == nil {
			return nil
		}

		existing, _ := s.database.GetJournal(ctx, event.SessionID)
		if existing != nil {
			return nil
		}

		time.Sleep(5 * time.Second)

		summaryKey := "sessions/" + event.SessionID + "/summary.json"
		summaryData, err := s.gold.GetObject(ctx, summaryKey)
		if err != nil {
			slog.Debug("journal worker: summary not ready", "session", event.SessionID)
			return nil
		}

		var summary struct {
			CarCode     int32  `json:"car_code"`
			TrackName   string `json:"track_name"`
			LapCount    int    `json:"lap_count"`
			Consistency struct {
				ConsistencyScore float64 `json:"consistency_score"`
				BestLapIdx       int     `json:"best_lap_idx"`
				WorstLapIdx      int     `json:"worst_lap_idx"`
				BestWorstDeltaMs int     `json:"best_worst_delta_ms"`
			} `json:"consistency"`
			TyreDegradation struct {
				DegradationRate float64 `json:"degradation_rate"`
				CompoundGuess   string  `json:"compound_guess"`
			} `json:"tyre_degradation"`
			Journal struct {
				BestLapMs      int      `json:"best_lap_ms"`
				WorstLapMs     int      `json:"worst_lap_ms"`
				Highlights     []string `json:"highlights"`
				AreasToImprove []string `json:"areas_to_improve"`
				CornerNotes    []string `json:"corner_notes"`
			} `json:"journal"`
		}
		if err := json.Unmarshal(summaryData, &summary); err != nil {
			slog.Error("journal worker: parse summary", "session", event.SessionID, "err", err)
			return nil
		}

		prompt := buildJournalPrompt(summary.TrackName, summary.CarCode, summary.LapCount,
			summary.Consistency.ConsistencyScore, summary.Consistency.BestWorstDeltaMs,
			summary.Consistency.BestLapIdx, summary.Consistency.WorstLapIdx,
			summary.Journal.BestLapMs, summary.Journal.WorstLapMs,
			summary.TyreDegradation.DegradationRate, summary.TyreDegradation.CompoundGuess,
			summary.Journal.Highlights, summary.Journal.AreasToImprove, summary.Journal.CornerNotes,
			s.cars)

		content, err := s.llm.GenerateJournal(ctx, prompt)
		if err != nil {
			slog.Error("journal worker: llm generation", "session", event.SessionID, "err", err)
			return nil
		}

		journal := &db.Journal{
			SessionID: event.SessionID,
			Content:   content,
		}
		if err := s.database.SaveJournal(ctx, journal); err != nil {
			slog.Error("journal worker: save", "session", event.SessionID, "err", err)
			return nil
		}

		slog.Info("journal auto-generated", "session", event.SessionID)
		s.sendPushNotification("Session Complete", fmt.Sprintf("Session %s finished with %d laps", event.SessionID, event.LapCount))
		return nil
	})
}

func buildBriefingPrompt(track string, carCode int32, lapCount int,
	consistency, lapTimeCV float64, bestWorstDelta int,
	bestLapMs, worstLapMs int,
	tyreDegrad float64, compound string, frBalance float64,
	fuelPerLap float64, fuelLapsRemain, pitLap int,
	highlights, areasToImprove, cornerNotes []string,
	history []db.SessionHistory, cars []carEntry) string {

	var b strings.Builder
	b.WriteString("Generate a pre-race briefing for the driver. Be concise, actionable, and specific.\n\n")
	b.WriteString("SESSION DATA:\n")
	b.WriteString("Track: " + track + "\n")
	carName := "Car " + strconv.Itoa(int(carCode))
	for _, c := range cars {
		if c.ID == int(carCode) {
			carName = c.Name + " (" + c.Maker + ")"
			break
		}
	}
	b.WriteString("Car: " + carName + "\n")
	b.WriteString("Laps completed: " + strconv.Itoa(lapCount) + "\n")
	b.WriteString("Best lap: " + formatLapTimeMs(bestLapMs) + "\n")
	b.WriteString("Worst lap: " + formatLapTimeMs(worstLapMs) + "\n")
	b.WriteString("Consistency score: " + strconv.FormatFloat(consistency*100, 'f', 1, 64) + "%\n")
	b.WriteString("Lap time CV: " + strconv.FormatFloat(lapTimeCV*100, 'f', 2, 64) + "%\n")
	b.WriteString("Best-worst delta: " + strconv.Itoa(bestWorstDelta) + "ms\n\n")

	b.WriteString("TYRE:\n")
	b.WriteString("Compound: " + compound + "\n")
	b.WriteString("Degradation rate: " + strconv.FormatFloat(tyreDegrad, 'f', 2, 64) + " C/lap\n")
	b.WriteString("Front/rear balance: " + strconv.FormatFloat(frBalance, 'f', 2, 64) + "\n\n")

	b.WriteString("FUEL:\n")
	b.WriteString("Consumption: " + strconv.FormatFloat(fuelPerLap, 'f', 2, 64) + " L/lap\n")
	b.WriteString("Range: " + strconv.Itoa(fuelLapsRemain) + " laps\n")
	if pitLap > 0 {
		b.WriteString("Optimal pit: lap " + strconv.Itoa(pitLap) + "\n")
	}
	b.WriteString("\n")

	if len(highlights) > 0 {
		b.WriteString("STRENGTHS:\n")
		for _, h := range highlights {
			b.WriteString("- " + h + "\n")
		}
		b.WriteString("\n")
	}

	if len(areasToImprove) > 0 {
		b.WriteString("AREAS TO IMPROVE:\n")
		for _, a := range areasToImprove {
			b.WriteString("- " + a + "\n")
		}
		b.WriteString("\n")
	}

	if len(cornerNotes) > 0 {
		b.WriteString("CORNER NOTES:\n")
		for _, n := range cornerNotes {
			b.WriteString("- " + n + "\n")
		}
		b.WriteString("\n")
	}

	if len(history) > 0 {
		b.WriteString("TRACK HISTORY (recent sessions on this track):\n")
		for _, h := range history {
			carName := "Car " + strconv.Itoa(int(h.CarCode))
			for _, c := range cars {
				if c.ID == int(h.CarCode) {
					carName = c.Name
					break
				}
			}
			lapStr := "--:--.---"
			if h.BestLapMs != nil {
				lapStr = formatLapTimeMs(int(*h.BestLapMs))
			}
			b.WriteString("- " + h.StartedAt.Format("2006-01-02") + " | " + carName + " | Best: " + lapStr + " | " + strconv.Itoa(h.LapCount) + " laps\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("INSTRUCTIONS:\n")
	b.WriteString("Write a focused pre-race briefing (3-5 paragraphs) covering:\n")
	b.WriteString("1. Target lap time and realistic expectations\n")
	b.WriteString("2. Key corners to focus on (reference specific weaknesses)\n")
	b.WriteString("3. Tyre management approach for this stint\n")
	b.WriteString("4. Fuel strategy if relevant\n")
	b.WriteString("5. One specific technique to practice this session\n")
	b.WriteString("Address the driver as 'you'. Be direct and motivating without being cheesy.")

	return b.String()
}

func buildJournalPrompt(track string, carCode int32, lapCount int,
	consistency float64, bestWorstDelta, bestLapIdx, worstLapIdx int,
	bestLapMs, worstLapMs int,
	tyreDegrad float64, compound string,
	highlights, areasToImprove, cornerNotes []string,
	cars []carEntry) string {

	var b strings.Builder
	b.WriteString("Generate a post-session journal entry for a Gran Turismo 7 driver.\n\n")
	b.WriteString("SESSION:\n")
	b.WriteString("Track: " + track + "\n")
	carName := "Car " + strconv.Itoa(int(carCode))
	for _, c := range cars {
		if c.ID == int(carCode) {
			carName = c.Name + " (" + c.Maker + ")"
			break
		}
	}
	b.WriteString("Car: " + carName + "\n")
	b.WriteString("Laps: " + strconv.Itoa(lapCount) + "\n")
	b.WriteString("Best: " + formatLapTimeMs(bestLapMs) + " (lap " + strconv.Itoa(bestLapIdx+1) + ")\n")
	b.WriteString("Worst: " + formatLapTimeMs(worstLapMs) + " (lap " + strconv.Itoa(worstLapIdx+1) + ")\n")
	b.WriteString("Consistency: " + strconv.FormatFloat(consistency*100, 'f', 1, 64) + "%\n")
	b.WriteString("Best-worst delta: " + strconv.Itoa(bestWorstDelta) + "ms\n")
	b.WriteString("Tyre degradation: " + strconv.FormatFloat(tyreDegrad, 'f', 2, 64) + " C/lap (" + compound + ")\n\n")

	if len(highlights) > 0 {
		b.WriteString("WHAT WENT WELL:\n")
		for _, h := range highlights {
			b.WriteString("- " + h + "\n")
		}
		b.WriteString("\n")
	}

	if len(areasToImprove) > 0 {
		b.WriteString("WHAT NEEDS WORK:\n")
		for _, a := range areasToImprove {
			b.WriteString("- " + a + "\n")
		}
		b.WriteString("\n")
	}

	if len(cornerNotes) > 0 {
		b.WriteString("CORNER OBSERVATIONS:\n")
		for _, n := range cornerNotes {
			b.WriteString("- " + n + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("INSTRUCTIONS:\n")
	b.WriteString("Write a concise session journal (2-4 paragraphs) that:\n")
	b.WriteString("1. Summarizes the session narrative (how it progressed, where breakthroughs happened)\n")
	b.WriteString("2. Identifies the key learning from this session\n")
	b.WriteString("3. Sets 1-2 specific goals for next time on this track\n")
	b.WriteString("Write in first person as if the driver is writing their own notes. Be honest and reflective.")

	return b.String()
}
