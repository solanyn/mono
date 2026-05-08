package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type TTSRequest struct {
	Text  string `json:"text"`
	Voice string `json:"voice,omitempty"`
	Speed float64 `json:"speed,omitempty"`
}

type OpenAITTSRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format"`
	Speed          float64 `json:"speed,omitempty"`
}

var (
	kokoroURL = envOr("KOKORO_URL", "http://mac.internal:8000")
	listenAddr = envOr("LISTEN_ADDR", ":8090")
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("client connected: %s", r.RemoteAddr)

	var mu sync.Mutex
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("read error: %v", err)
			}
			return
		}

		var req TTSRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			mu.Lock()
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"invalid json"}`))
			mu.Unlock()
			continue
		}

		if req.Text == "" {
			continue
		}

		go func(text, voice string, speed float64) {
			start := time.Now()

			if voice == "" {
				voice = "af_heart"
			}
			if speed == 0 {
				speed = 1.0
			}

			audio, err := callKokoroTTS(text, voice, speed)
			if err != nil {
				log.Printf("tts error: %v", err)
				mu.Lock()
				conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
				mu.Unlock()
				return
			}

			latency := time.Since(start)
			log.Printf("tts latency=%v text=%q bytes=%d", latency, text[:min(len(text), 50)], len(audio))

			mu.Lock()
			meta, _ := json.Marshal(map[string]interface{}{
				"type":       "audio",
				"format":     "wav",
				"latency_ms": latency.Milliseconds(),
				"text":       text,
			})
			conn.WriteMessage(websocket.TextMessage, meta)
			conn.WriteMessage(websocket.BinaryMessage, audio)
			mu.Unlock()
		}(req.Text, req.Voice, req.Speed)
	}
}

func callKokoroTTS(text, voice string, speed float64) ([]byte, error) {
	payload := OpenAITTSRequest{
		Model:          "kokoro",
		Input:          text,
		Voice:          voice,
		ResponseFormat: "wav",
		Speed:          speed,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(kokoroURL+"/v1/audio/speech", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("kokoro request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kokoro returned %d: %s", resp.StatusCode, string(errBody))
	}

	return io.ReadAll(resp.Body)
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Printf("audio proxy listening on %s (kokoro: %s)", listenAddr, kokoroURL)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Fatal(err)
	}
}
