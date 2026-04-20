package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"
)

const (
	audioHTTPTimeout = 10 * time.Minute
	chunkDuration    = 2 * time.Minute
)

var audioClient = &http.Client{Timeout: audioHTTPTimeout}

type VADSegment struct {
	Speaker string  `json:"speaker"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
}

type VADResult struct {
	Segments        []VADSegment `json:"segments"`
	NumSpeakers     int          `json:"num_speakers"`
	DurationSeconds float64      `json:"duration_seconds"`
}

type TranscriptWord struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type TranscriptResult struct {
	Text  string           `json:"text"`
	Words []TranscriptWord `json:"words,omitempty"`
}

func (s *Server) vad(ctx context.Context, audioData io.Reader, filename string, threshold string) (*VADResult, error) {
	body, ct, err := multipartFromReader(audioData, filename, map[string]string{
		"threshold": threshold,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.AudioServiceURL+"/v1/audio/vad", body)
	if err != nil {
		return nil, fmt.Errorf("vad request: %w", err)
	}
	req.Header.Set("Content-Type", ct)
	resp, err := audioClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vad request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vad %d: %s", resp.StatusCode, b)
	}

	var result VADResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("vad decode: %w", err)
	}
	return &result, nil
}

func (s *Server) transcribe(ctx context.Context, audioData io.Reader, filename string) (*TranscriptResult, error) {
	body, ct, err := multipartFromReader(audioData, filename, map[string]string{
		"model":           s.cfg.STTModel,
		"response_format": "verbose_json",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.AudioServiceURL+"/v1/audio/transcriptions", body)
	if err != nil {
		return nil, fmt.Errorf("transcribe request: %w", err)
	}
	req.Header.Set("Content-Type", ct)
	resp, err := audioClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transcribe request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("transcribe %d: %s", resp.StatusCode, b)
	}

	var result TranscriptResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("transcribe decode: %w", err)
	}
	return &result, nil
}

type wavHeader struct {
	SampleRate    uint32
	NumChannels   uint16
	BitsPerSample uint16
	DataOffset    int
	DataSize      int
}

func parseWAVHeader(data []byte) (*wavHeader, error) {
	if len(data) < 44 {
		return nil, fmt.Errorf("wav too short: %d bytes", len(data))
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a WAV file")
	}

	offset := 12
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		if chunkID == "fmt " {
			if chunkSize < 16 {
				return nil, fmt.Errorf("fmt chunk too small")
			}
			h := &wavHeader{
				NumChannels:   binary.LittleEndian.Uint16(data[offset+10 : offset+12]),
				SampleRate:    binary.LittleEndian.Uint32(data[offset+12 : offset+16]),
				BitsPerSample: binary.LittleEndian.Uint16(data[offset+22 : offset+24]),
			}
			offset += 8 + chunkSize
			if chunkSize%2 != 0 {
				offset++
			}
			for offset+8 <= len(data) {
				id := string(data[offset : offset+4])
				sz := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
				if id == "data" {
					h.DataOffset = offset + 8
					h.DataSize = sz
					return h, nil
				}
				offset += 8 + sz
				if sz%2 != 0 {
					offset++
				}
			}
			return nil, fmt.Errorf("no data chunk found")
		}
		offset += 8 + chunkSize
		if chunkSize%2 != 0 {
			offset++
		}
	}
	return nil, fmt.Errorf("no fmt chunk found")
}

func buildWAVChunk(h *wavHeader, pcmData []byte) []byte {
	dataSize := len(pcmData)
	fileSize := 36 + dataSize
	buf := make([]byte, 44+dataSize)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(fileSize))
	copy(buf[8:12], "WAVE")

	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(buf[22:24], h.NumChannels)
	binary.LittleEndian.PutUint32(buf[24:28], h.SampleRate)
	blockAlign := h.NumChannels * h.BitsPerSample / 8
	binary.LittleEndian.PutUint32(buf[28:32], h.SampleRate*uint32(blockAlign))
	binary.LittleEndian.PutUint16(buf[32:34], blockAlign)
	binary.LittleEndian.PutUint16(buf[34:36], h.BitsPerSample)

	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))
	copy(buf[44:], pcmData)

	return buf
}

func splitWAVChunks(data []byte, chunkSecs float64) ([][]byte, error) {
	h, err := parseWAVHeader(data)
	if err != nil {
		return nil, err
	}

	blockAlign := int(h.NumChannels) * int(h.BitsPerSample) / 8
	bytesPerSec := int(h.SampleRate) * blockAlign
	chunkBytes := int(chunkSecs) * bytesPerSec

	pcmStart := h.DataOffset
	pcmEnd := pcmStart + h.DataSize
	if pcmEnd > len(data) {
		pcmEnd = len(data)
	}

	var chunks [][]byte
	for off := pcmStart; off < pcmEnd; off += chunkBytes {
		end := off + chunkBytes
		if end > pcmEnd {
			end = pcmEnd
		}
		aligned := ((end - off) / blockAlign) * blockAlign
		chunks = append(chunks, buildWAVChunk(h, data[off:off+aligned]))
	}
	return chunks, nil
}

func (s *Server) vadChunked(ctx context.Context, audioData []byte, filename string, threshold string) (*VADResult, error) {
	chunkSecs := chunkDuration.Seconds()
	chunks, err := splitWAVChunks(audioData, chunkSecs)
	if err != nil {
		return nil, fmt.Errorf("split wav: %w", err)
	}

	h, _ := parseWAVHeader(audioData)
	blockAlign := int(h.NumChannels) * int(h.BitsPerSample) / 8
	bytesPerSec := float64(h.SampleRate) * float64(blockAlign)

	var allSegments []VADSegment
	totalDuration := float64(h.DataSize) / bytesPerSec
	var numSpeakers int

	for i, chunk := range chunks {
		timeOffset := float64(i) * chunkSecs
		log.Printf("vad chunk %d/%d (offset %.0fs, %d bytes)", i+1, len(chunks), timeOffset, len(chunk))

		result, err := s.vad(ctx, bytes.NewReader(chunk), filename, threshold)
		if err != nil {
			return nil, fmt.Errorf("vad chunk %d: %w", i+1, err)
		}

		for _, seg := range result.Segments {
			allSegments = append(allSegments, VADSegment{
				Speaker: seg.Speaker,
				Start:   seg.Start + timeOffset,
				End:     seg.End + timeOffset,
			})
		}
		if result.NumSpeakers > numSpeakers {
			numSpeakers = result.NumSpeakers
		}
	}

	return &VADResult{
		Segments:        allSegments,
		NumSpeakers:     numSpeakers,
		DurationSeconds: totalDuration,
	}, nil
}

func multipartFromReader(r io.Reader, filename string, fields map[string]string) (io.Reader, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range fields {
		w.WriteField(k, v)
	}

	fw, err := w.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(fw, r); err != nil {
		return nil, "", err
	}
	w.Close()

	return &buf, w.FormDataContentType(), nil
}
