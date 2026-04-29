// Package client implements the scrib-server HTTP client used by the TUI
// and the `scrib upload` resume path. It owns retry, per-attempt timeouts,
// and progress reporting so the TUI stays a dumb renderer.
package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Progress is emitted after each successful stage. TUI turns each event
// into a "step done, next step in flight" transition.
type Progress struct {
	Stage   string // "metadata" | "audio" | "process"
	Attempt int    // 1-based
	Err     error  // non-nil when this attempt failed (next attempt will retry)
}

// ProgressFunc is called from the uploader's own goroutine. Implementations
// must be goroutine-safe; the TUI typically forwards to a tea.Program.Send.
type ProgressFunc func(Progress)

// UploadInput is everything the uploader needs to push a recording to the server.
type UploadInput struct {
	ServerURL string
	Name      string
	Template  string
	Duration  time.Duration

	// WAVPath is the on-disk mono WAV to upload. The uploader opens it per
	// attempt so the body can be replayed.
	WAVPath string
}

// UploadResult is returned on success.
type UploadResult struct {
	UUID string
}

const (
	// Metadata + process are small, short-lived RPCs.
	metadataTimeout = 30 * time.Second
	processTimeout  = 30 * time.Second
	// Audio upload can legitimately take minutes over flaky wifi.
	audioTimeout = 15 * time.Minute

	retryAttempts = 3
	retryBase     = 2 * time.Second
)

// Upload runs metadata → audio → process with per-stage retry. Progress
// events are emitted after every attempt (success or failure); the caller
// sees each stage advance and can tell which attempt the uploader is on.
func Upload(ctx context.Context, in UploadInput, progress ProgressFunc) (*UploadResult, error) {
	if in.ServerURL == "" {
		return nil, errors.New("server URL empty")
	}
	if _, err := os.Stat(in.WAVPath); err != nil {
		return nil, fmt.Errorf("wav not readable: %w", err)
	}

	var uuid string
	if err := withRetry(ctx, "metadata", metadataTimeout, progress, func(ctx context.Context) error {
		id, err := postMeeting(ctx, in)
		if err != nil {
			return err
		}
		uuid = id
		return nil
	}); err != nil {
		return nil, fmt.Errorf("metadata: %w", err)
	}

	if err := withRetry(ctx, "audio", audioTimeout, progress, func(ctx context.Context) error {
		return postAudio(ctx, in.ServerURL, uuid, in.WAVPath)
	}); err != nil {
		return nil, fmt.Errorf("audio: %w", err)
	}

	if err := withRetry(ctx, "process", processTimeout, progress, func(ctx context.Context) error {
		return postProcess(ctx, in.ServerURL, uuid)
	}); err != nil {
		return nil, fmt.Errorf("process: %w", err)
	}

	return &UploadResult{UUID: uuid}, nil
}

func withRetry(ctx context.Context, stage string, timeout time.Duration, progress ProgressFunc, fn func(context.Context) error) error {
	var lastErr error
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		if attempt > 1 {
			wait := retryBase << (attempt - 2)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		err := fn(attemptCtx)
		cancel()

		if progress != nil {
			progress(Progress{Stage: stage, Attempt: attempt, Err: err})
		}
		if err == nil {
			return nil
		}
		// Don't keep banging on a cancelled parent context.
		if errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}
		lastErr = err
	}
	return lastErr
}

func postMeeting(ctx context.Context, in UploadInput) (string, error) {
	body := map[string]any{
		"name":        in.Name,
		"recorded_at": time.Now().Format(time.RFC3339),
		"duration_s":  in.Duration.Seconds(),
		"template":    in.Template,
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", in.ServerURL+"/v1/meetings", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		var result struct {
			UUID string `json:"uuid"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result.UUID != "" {
			return result.UUID, nil
		}
	}

	// Fallback to /v1/sync/push with a client-generated UUID.
	uuid, err := newUUID()
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"client_id": "scrib-client",
		"meetings": []map[string]any{{
			"uuid":         uuid,
			"name":         in.Name,
			"recorded_at":  time.Now().Format(time.RFC3339),
			"duration_s":   in.Duration.Seconds(),
			"template":     in.Template,
			"num_speakers": 0,
			"segments":     []any{},
			"summaries":    []any{},
		}},
	}
	data, _ = json.Marshal(payload)

	req2, err := http.NewRequestWithContext(ctx, "POST", in.ServerURL+"/v1/sync/push", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		return "", err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp2.Body)
		return "", fmt.Errorf("server returned %d: %s", resp2.StatusCode, b)
	}
	return uuid, nil
}

func postAudio(ctx context.Context, serverURL, uuid, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/v1/audio/"+uuid, f)
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

func postProcess(ctx context.Context, serverURL, uuid string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/v1/process/"+uuid, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
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

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
