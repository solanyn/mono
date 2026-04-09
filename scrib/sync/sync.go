// Package sync provides client-side sync logic for scrib.
package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/solanyn/mono/scrib/store"
)

// Client handles syncing local SQLite to the scrib server.
type Client struct {
	serverURL  string
	clientID   string
	httpClient *http.Client
	db         *store.DB
}

// NewClient creates a sync client.
func NewClient(serverURL, clientID string, db *store.DB) *Client {
	return &Client{
		serverURL:  serverURL,
		clientID:   clientID,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		db:         db,
	}
}

// PushPayload matches the server's SyncPayload.
type PushPayload struct {
	ClientID string        `json:"client_id"`
	Meetings []PushMeeting `json:"meetings"`
	Speakers []PushSpeaker `json:"speakers,omitempty"`
}

type PushMeeting struct {
	UUID        string        `json:"uuid"`
	Name        string        `json:"name"`
	RecordedAt  time.Time     `json:"recorded_at"`
	DurationS   float64       `json:"duration_s"`
	Template    string        `json:"template"`
	NumSpeakers int           `json:"num_speakers"`
	Segments    []PushSegment `json:"segments"`
	Summaries   []PushSummary `json:"summaries"`
}

type PushSegment struct {
	UUID         string  `json:"uuid"`
	SpeakerLabel string  `json:"speaker_label"`
	StartS       float64 `json:"start_s"`
	EndS         float64 `json:"end_s"`
	Text         string  `json:"text"`
}

type PushSummary struct {
	UUID     string `json:"uuid"`
	Template string `json:"template"`
	Content  string `json:"content"`
}

type PushSpeaker struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	Embedding []byte `json:"embedding,omitempty"`
}

type PushResponse struct {
	Synced  int    `json:"synced"`
	Cursor  string `json:"cursor"`
	Message string `json:"message,omitempty"`
}

// Push sends unsynced meetings + audio to the server.
func (c *Client) Push() (*PushResponse, error) {
	// Get unsynced meetings
	meetings, err := c.db.UnsyncedMeetings()
	if err != nil {
		return nil, fmt.Errorf("get unsynced: %w", err)
	}

	if len(meetings) == 0 {
		return &PushResponse{Message: "nothing to sync"}, nil
	}

	// Get speakers
	speakers, _ := c.db.ListSpeakers()

	payload := PushPayload{ClientID: c.clientID}

	for _, sp := range speakers {
		payload.Speakers = append(payload.Speakers, PushSpeaker{
			UUID: sp.UUID, Name: sp.Name, Embedding: sp.Embedding,
		})
	}

	for _, m := range meetings {
		pm := PushMeeting{
			UUID: m.UUID, Name: m.Name, RecordedAt: m.RecordedAt,
			DurationS: m.DurationS, Template: m.Template, NumSpeakers: m.NumSpeakers,
		}

		segments, _ := c.db.UnsyncedSegments(m.ID)
		for _, s := range segments {
			pm.Segments = append(pm.Segments, PushSegment{
				UUID: s.UUID, SpeakerLabel: s.SpeakerLabel,
				StartS: s.StartS, EndS: s.EndS, Text: s.Text,
			})
		}

		summaries, _ := c.db.UnsyncedSummaries(m.ID)
		for _, s := range summaries {
			pm.Summaries = append(pm.Summaries, PushSummary{
				UUID: s.UUID, Template: s.Template, Content: s.Content,
			})
		}

		payload.Meetings = append(payload.Meetings, pm)
	}

	// Push structured data
	body, _ := json.Marshal(payload)
	resp, err := c.httpClient.Post(c.serverURL+"/v1/sync/push", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("push: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("push failed (%d): %s", resp.StatusCode, b)
	}

	var result PushResponse
	json.NewDecoder(resp.Body).Decode(&result)

	// Mark synced
	for _, m := range meetings {
		c.db.MarkMeetingSynced(m.UUID)

		// Upload audio if exists
		if m.AudioPath != "" {
			if err := c.uploadAudio(m.UUID, m.AudioPath); err != nil {
				fmt.Fprintf(os.Stderr, "warning: audio upload for %s: %v\n", m.UUID, err)
			}
		}
	}

	return &result, nil
}

func (c *Client) uploadAudio(uuid, audioPath string) error {
	f, err := os.Open(audioPath)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, _ := f.Stat()
	req, err := http.NewRequest("POST", c.serverURL+"/v1/audio/"+uuid, f)
	if err != nil {
		return err
	}
	req.ContentLength = stat.Size()
	req.Header.Set("Content-Type", "audio/wav")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed (%d): %s", resp.StatusCode, b)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if key, ok := result["blob_key"]; ok {
		c.db.MarkMeetingBlobKey(uuid, key)
	}

	return nil
}

// Pull fetches new meetings from the server.
func (c *Client) Pull() (int, error) {
	// TODO: read cursor from sync_state table
	resp, err := c.httpClient.Get(c.serverURL + "/v1/sync/pull?limit=50")
	if err != nil {
		return 0, fmt.Errorf("pull: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Meetings []PushMeeting `json:"meetings"`
		Cursor   string        `json:"cursor"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	synced := 0
	for _, m := range result.Meetings {
		err := c.db.UpsertFromSync(&store.Meeting{
			UUID: m.UUID, Name: m.Name, RecordedAt: m.RecordedAt,
			DurationS: m.DurationS, Template: m.Template, NumSpeakers: m.NumSpeakers,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "pull meeting %s: %v\n", m.UUID, err)
			continue
		}

		for _, seg := range m.Segments {
			c.db.UpsertSegmentFromSync(&store.Segment{
				UUID: seg.UUID, SpeakerLabel: seg.SpeakerLabel,
				StartS: seg.StartS, EndS: seg.EndS, Text: seg.Text,
			}, m.UUID)
		}

		for _, sum := range m.Summaries {
			c.db.UpsertSummaryFromSync(&store.Summary{
				UUID: sum.UUID, Template: sum.Template, Content: sum.Content,
			}, m.UUID)
		}
		synced++
	}

	return synced, nil
}
