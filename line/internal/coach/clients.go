package coach

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type TTSClient struct {
	endpoint   string
	httpClient *http.Client
}

func NewTTSClient(endpoint string) *TTSClient {
	return &TTSClient{
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *TTSClient) Synthesize(ctx context.Context, text, voice string, speed float64) ([]byte, error) {
	body := map[string]interface{}{
		"model": "kokoro",
		"input": text,
		"voice": voice,
		"speed": speed,
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/v1/audio/speech", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("tts request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tts status: %d", resp.StatusCode)
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tts read: %w", err)
	}
	return audio, nil
}

type LLMClient struct {
	endpoint   string
	model      string
	httpClient *http.Client
}

func NewLLMClient(endpoint, model string) *LLMClient {
	return &LLMClient{
		endpoint:   endpoint,
		model:      model,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

const systemPrompt = `You are a concise racing coach for Gran Turismo 7. 
Given telemetry events, provide brief, actionable coaching advice in 1-2 sentences.
Be direct and specific. Use racing terminology naturally.
Never say "I" or refer to yourself. Address the driver as "you".`

func (c *LLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	body := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm status: %d", resp.StatusCode)
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("llm decode: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("llm: no choices returned")
	}
	return result.Choices[0].Message.Content, nil
}

const briefingSystemPrompt = `You are a professional racing engineer preparing a pre-race briefing for a Gran Turismo 7 driver.
Write clear, structured, actionable briefings based on telemetry data from previous sessions.
Be specific about corner techniques, braking points, and strategy.
Address the driver as "you". Be direct and motivating without being cheesy.
Never say "I" or refer to yourself.`

const journalSystemPrompt = `You are writing a personal racing journal for a Gran Turismo 7 driver.
Write reflective, honest entries that capture the session experience.
Use first person. Be specific about what happened and what was learned.
Keep it concise but insightful. No fluff.`

func (c *LLMClient) GenerateJournal(ctx context.Context, prompt string) (string, error) {
	body := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: journalSystemPrompt},
			{Role: "user", Content: prompt},
		},
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("llm journal request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm journal call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm journal status: %d", resp.StatusCode)
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("llm journal decode: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("llm journal: no choices returned")
	}
	return result.Choices[0].Message.Content, nil
}

func (c *LLMClient) GenerateBriefing(ctx context.Context, prompt string) (string, error) {
	body := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: briefingSystemPrompt},
			{Role: "user", Content: prompt},
		},
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("llm briefing request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm briefing call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm briefing status: %d", resp.StatusCode)
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("llm briefing decode: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("llm briefing: no choices returned")
	}
	return result.Choices[0].Message.Content, nil
}
