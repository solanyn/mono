package coach

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Pipeline struct {
	detector *Detector
	tts      *TTSClient
	llm      *LLMClient
	voice    string
	speed    float64
	useLLM   bool

	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
}

type Config struct {
	TTSEndpoint string
	LLMEndpoint string
	LLMModel    string
	Voice       string
	Speed       float64
	UseLLM      bool
}

func NewPipeline(cfg Config) *Pipeline {
	if cfg.Voice == "" {
		cfg.Voice = "af_heart"
	}
	if cfg.Speed == 0 {
		cfg.Speed = 1.0
	}

	p := &Pipeline{
		detector: NewDetector(64),
		tts:      NewTTSClient(cfg.TTSEndpoint),
		voice:    cfg.Voice,
		speed:    cfg.Speed,
		useLLM:   cfg.UseLLM,
		clients:  make(map[*websocket.Conn]struct{}),
	}

	if cfg.LLMEndpoint != "" && cfg.UseLLM {
		p.llm = NewLLMClient(cfg.LLMEndpoint, cfg.LLMModel)
	}

	return p
}

func (p *Pipeline) Detector() *Detector {
	return p.detector
}

func (p *Pipeline) AddClient(conn *websocket.Conn) {
	p.mu.Lock()
	p.clients[conn] = struct{}{}
	p.mu.Unlock()
}

func (p *Pipeline) RemoveClient(conn *websocket.Conn) {
	p.mu.Lock()
	delete(p.clients, conn)
	p.mu.Unlock()
}

func (p *Pipeline) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-p.detector.Events():
			go p.handleEvent(ctx, event)
		}
	}
}

func (p *Pipeline) handleEvent(ctx context.Context, event Event) {
	start := time.Now()

	prompt := FormatPrompt(event)
	if prompt == "" {
		return
	}

	text := prompt
	if p.useLLM && p.llm != nil {
		generated, err := p.llm.Generate(ctx, prompt)
		if err != nil {
			slog.Error("llm generate", "err", err, "event", event.Type)
		} else {
			text = generated
		}
	}

	audio, err := p.tts.Synthesize(ctx, text, p.voice, p.speed)
	if err != nil {
		slog.Error("tts synthesize", "err", err, "event", event.Type)
		return
	}

	latency := time.Since(start)
	slog.Info("coach event", "type", event.Type, "latency", latency, "text_len", len(text), "audio_bytes", len(audio))

	p.broadcast(text, audio, latency)
}

type coachMeta struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	LatencyMs int64  `json:"latency_ms"`
}

func (p *Pipeline) broadcast(text string, audio []byte, latency time.Duration) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	meta, _ := json.Marshal(coachMeta{
		Type:      "audio",
		Text:      text,
		LatencyMs: latency.Milliseconds(),
	})

	for conn := range p.clients {
		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, meta); err != nil {
			conn.Close()
			go p.RemoveClient(conn)
			continue
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, audio); err != nil {
			conn.Close()
			go p.RemoveClient(conn)
		}
	}
}
