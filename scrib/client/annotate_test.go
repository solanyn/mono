package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestAlignWordFullyInsideSegment(t *testing.T) {
	vad := &VADResult{
		Segments: []VADSegment{
			{Speaker: "SPEAKER_0", Start: 0.0, End: 5.0},
			{Speaker: "SPEAKER_1", Start: 5.0, End: 10.0},
		},
		DurationSeconds: 10.0,
	}
	stt := &TranscriptResult{
		Text: "hello",
		Words: []TranscriptWord{
			{Word: "hello", Start: 1.0, End: 1.5},
		},
	}

	segs := alignSpeakersToWords(vad, stt)
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if segs[0].Speaker != "SPEAKER_0" {
		t.Errorf("speaker = %s, want SPEAKER_0", segs[0].Speaker)
	}
	if segs[0].Uncertain {
		t.Error("should not be uncertain")
	}
}

func TestAlignWordSpanningTwoSegments(t *testing.T) {
	vad := &VADResult{
		Segments: []VADSegment{
			{Speaker: "SPEAKER_0", Start: 0.0, End: 5.0},
			{Speaker: "SPEAKER_1", Start: 5.0, End: 10.0},
		},
		DurationSeconds: 10.0,
	}
	stt := &TranscriptResult{
		Text: "boundary",
		Words: []TranscriptWord{
			{Word: "boundary", Start: 4.0, End: 6.0},
		},
	}

	segs := alignSpeakersToWords(vad, stt)
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if segs[0].Speaker != "SPEAKER_0" {
		t.Errorf("speaker = %s, want SPEAKER_0 (more overlap)", segs[0].Speaker)
	}
}

func TestAlignWordNoOverlapFallsBackToNearest(t *testing.T) {
	vad := &VADResult{
		Segments: []VADSegment{
			{Speaker: "SPEAKER_0", Start: 0.0, End: 2.0},
			{Speaker: "SPEAKER_1", Start: 8.0, End: 10.0},
		},
		DurationSeconds: 10.0,
	}
	stt := &TranscriptResult{
		Text: "gap",
		Words: []TranscriptWord{
			{Word: "gap", Start: 4.0, End: 4.5},
		},
	}

	segs := alignSpeakersToWords(vad, stt)
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if segs[0].Speaker != "SPEAKER_0" {
		t.Errorf("speaker = %s, want SPEAKER_0 (nearest)", segs[0].Speaker)
	}
	if !segs[0].Uncertain {
		t.Error("should be uncertain (no overlap)")
	}
}

func TestAlignBoundaryWordLowOverlapUncertain(t *testing.T) {
	vad := &VADResult{
		Segments: []VADSegment{
			{Speaker: "SPEAKER_0", Start: 0.0, End: 5.1},
			{Speaker: "SPEAKER_1", Start: 5.5, End: 10.0},
		},
		DurationSeconds: 10.0,
	}
	stt := &TranscriptResult{
		Text: "edge",
		Words: []TranscriptWord{
			{Word: "edge", Start: 4.9, End: 6.0},
		},
	}

	segs := alignSpeakersToWords(vad, stt)
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}

	wordDur := 6.0 - 4.9
	overlapA := 5.1 - 4.9
	overlapB := 6.0 - 5.5
	bestOverlap := overlapA
	if overlapB > overlapA {
		bestOverlap = overlapB
	}
	ratio := bestOverlap / wordDur
	if ratio >= 0.5 {
		t.Skipf("overlap ratio %.2f >= 0.5, test setup wrong", ratio)
	}
	if !segs[0].Uncertain {
		t.Error("should be uncertain (overlap ratio < 0.5)")
	}
}

func TestAlignMultipleWordsMultipleSpeakers(t *testing.T) {
	vad := &VADResult{
		Segments: []VADSegment{
			{Speaker: "SPEAKER_0", Start: 0.0, End: 3.0},
			{Speaker: "SPEAKER_1", Start: 3.0, End: 6.0},
		},
		DurationSeconds: 6.0,
	}
	stt := &TranscriptResult{
		Text: "hello world foo bar",
		Words: []TranscriptWord{
			{Word: "hello", Start: 0.5, End: 1.0},
			{Word: "world", Start: 1.5, End: 2.0},
			{Word: "foo", Start: 3.5, End: 4.0},
			{Word: "bar", Start: 4.5, End: 5.0},
		},
	}

	segs := alignSpeakersToWords(vad, stt)
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	if segs[0].Speaker != "SPEAKER_0" || segs[0].Text != "hello world" {
		t.Errorf("seg[0] = %+v", segs[0])
	}
	if segs[1].Speaker != "SPEAKER_1" || segs[1].Text != "foo bar" {
		t.Errorf("seg[1] = %+v", segs[1])
	}
}

func TestAlignEmptySegments(t *testing.T) {
	vad := &VADResult{Segments: nil, DurationSeconds: 5.0}
	stt := &TranscriptResult{Text: "hello", Words: []TranscriptWord{{Word: "hello", Start: 0, End: 1}}}

	segs := alignSpeakersToWords(vad, stt)
	if len(segs) != 1 || segs[0].Speaker != "SPEAKER_0" {
		t.Errorf("fallback failed: %+v", segs)
	}
}

func TestAnnotateSummaryFailurePreservesSegments(t *testing.T) {
	vadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(VADResult{
			Segments:        []VADSegment{{Speaker: "SPEAKER_0", Start: 0, End: 5}},
			NumSpeakers:     1,
			DurationSeconds: 5.0,
		})
	}))
	defer vadSrv.Close()

	sttSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(TranscriptResult{
			Text:  "hello world",
			Words: []TranscriptWord{{Word: "hello", Start: 0, End: 1}, {Word: "world", Start: 1, End: 2}},
		})
	}))
	defer sttSrv.Close()

	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
	}))
	defer gatewaySrv.Close()

	tmpFile := t.TempDir() + "/test.wav"
	writeTestWAV(t, tmpFile)

	c := &Client{
		AudioURL:   vadSrv.URL,
		GatewayURL: gatewaySrv.URL,
		STTModel:   "test-model",
		HTTPClient: &http.Client{},
	}
	c.AudioURL = sttSrv.URL

	result, err := c.Annotate(context.Background(), tmpFile, 0.5, "standup")
	if err != nil {
		t.Fatalf("Annotate() should not return error on summary failure, got: %v", err)
	}
	if result.SummaryErr == nil {
		t.Fatal("expected SummaryErr to be set")
	}
	if len(result.Segments) == 0 {
		t.Fatal("expected segments to be preserved")
	}
	if result.Summary != "" {
		t.Errorf("expected empty summary, got %q", result.Summary)
	}
}

func TestAnnotateSuccessHasNoSummaryErr(t *testing.T) {
	audioSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/audio/vad":
			json.NewEncoder(w).Encode(VADResult{
				Segments:        []VADSegment{{Speaker: "SPEAKER_0", Start: 0, End: 5}},
				NumSpeakers:     1,
				DurationSeconds: 5.0,
			})
		case r.URL.Path == "/v1/audio/transcriptions":
			json.NewEncoder(w).Encode(TranscriptResult{
				Text:  "hello",
				Words: []TranscriptWord{{Word: "hello", Start: 0, End: 1}},
			})
		}
	}))
	defer audioSrv.Close()

	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "summary text"}},
			},
		})
	}))
	defer gatewaySrv.Close()

	tmpFile := t.TempDir() + "/test.wav"
	writeTestWAV(t, tmpFile)

	c := &Client{
		AudioURL:   audioSrv.URL,
		GatewayURL: gatewaySrv.URL,
		STTModel:   "test-model",
		HTTPClient: &http.Client{},
	}

	result, err := c.Annotate(context.Background(), tmpFile, 0.5, "standup")
	if err != nil {
		t.Fatalf("Annotate() error: %v", err)
	}
	if result.SummaryErr != nil {
		t.Errorf("unexpected SummaryErr: %v", result.SummaryErr)
	}
	if result.Summary != "summary text" {
		t.Errorf("summary = %q, want %q", result.Summary, "summary text")
	}
}

func TestAnnotateCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	tmpFile := t.TempDir() + "/test.wav"
	writeTestWAV(t, tmpFile)

	c := &Client{AudioURL: srv.URL, GatewayURL: srv.URL, STTModel: "test", HTTPClient: &http.Client{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Annotate(ctx, tmpFile, 0.5, "standup")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestAnnotateTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	tmpFile := t.TempDir() + "/test.wav"
	writeTestWAV(t, tmpFile)

	c := &Client{AudioURL: srv.URL, GatewayURL: srv.URL, STTModel: "test", HTTPClient: &http.Client{}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := c.Annotate(ctx, tmpFile, 0.5, "standup")
	if err == nil {
		t.Fatal("expected error from timeout")
	}
}

func writeTestWAV(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	header[16] = 16
	header[20] = 1
	header[22] = 1
	header[24] = 0x80
	header[25] = 0x3E
	header[32] = 2
	header[34] = 16
	copy(header[36:40], "data")
	f.Write(header)
}
