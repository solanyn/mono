// Package client implements the scrib-server HTTP client used by the TUI
// and the `scrib upload` resume path. It owns retry, per-attempt timeouts,
// and progress reporting so the TUI stays a dumb renderer.
package client

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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

// StreamEvent is one decoded SSE event from /v1/process/{uuid}/events.
// Stage comes from the `event:` line, Data is the raw JSON payload from `data:`.
type StreamEvent struct {
	Stage string
	Data  []byte
}

// StreamProcess subscribes to the server's SSE feed for a single meeting and
// forwards every event to onEvent. It returns when the server closes the
// connection (stage=done or stage=error) or when ctx is cancelled.
//
// Uses its own http.Client without a global timeout so the long-lived
// connection can block on the event stream.
func StreamProcess(ctx context.Context, serverURL, uuid string, onEvent func(StreamEvent)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", serverURL+"/v1/process/"+uuid+"/events", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Processing already finished (bus closed + reaped) or never started.
		// Caller should treat as "no stream available" and fall back to polling.
		return errStreamUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("sse %d: %s", resp.StatusCode, b)
	}

	scanner := bufio.NewScanner(resp.Body)
	// SSE events can carry full result payloads; raise the max line size.
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	var stage string
	var dataBuf bytes.Buffer
	flush := func() {
		if stage == "" && dataBuf.Len() == 0 {
			return
		}
		data := append([]byte(nil), dataBuf.Bytes()...)
		onEvent(StreamEvent{Stage: stage, Data: data})
		stage = ""
		dataBuf.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		// Blank line = end of event.
		if line == "" {
			flush()
			continue
		}
		// Comments (keepalives) start with ':'.
		if strings.HasPrefix(line, ":") {
			continue
		}
		if v, ok := strings.CutPrefix(line, "event:"); ok {
			stage = strings.TrimSpace(v)
			continue
		}
		if v, ok := strings.CutPrefix(line, "data:"); ok {
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(strings.TrimPrefix(v, " "))
			continue
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return nil
}

// ErrStreamUnavailable is returned when the server has no active event bus
// for the given uuid — either processing is already done or never started.
var errStreamUnavailable = errors.New("sse stream unavailable")

// IsStreamUnavailable reports whether err indicates the server isn't
// currently streaming for this meeting.
func IsStreamUnavailable(err error) bool { return errors.Is(err, errStreamUnavailable) }
