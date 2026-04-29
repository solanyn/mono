package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// WriteWAV writes interleaved int16 samples as a WAV file.
func WriteWAV(path string, samples []int16, sampleRate, channels int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	dataSize := len(samples) * 2
	fileSize := 36 + dataSize

	// RIFF header
	f.Write([]byte("RIFF"))
	binary.Write(f, binary.LittleEndian, uint32(fileSize))
	f.Write([]byte("WAVE"))

	// fmt chunk
	f.Write([]byte("fmt "))
	binary.Write(f, binary.LittleEndian, uint32(16))          // chunk size
	binary.Write(f, binary.LittleEndian, uint16(1))            // PCM
	binary.Write(f, binary.LittleEndian, uint16(channels))     // channels
	binary.Write(f, binary.LittleEndian, uint32(sampleRate))   // sample rate
	byteRate := sampleRate * channels * 2
	binary.Write(f, binary.LittleEndian, uint32(byteRate))     // byte rate
	blockAlign := channels * 2
	binary.Write(f, binary.LittleEndian, uint16(blockAlign))   // block align
	binary.Write(f, binary.LittleEndian, uint16(16))           // bits per sample

	// data chunk
	f.Write([]byte("data"))
	binary.Write(f, binary.LittleEndian, uint32(dataSize))

	// Write samples
	return binary.Write(f, binary.LittleEndian, samples)
}

func StereoToMono(samples []int16) []int16 {
	frames := len(samples) / 2
	mono := make([]int16, frames)
	for i := 0; i < frames; i++ {
		mono[i] = int16((int32(samples[i*2]) + int32(samples[i*2+1])) / 2)
	}
	return mono
}

// WriteWAVTemp writes samples to a temporary WAV file and returns the path.
func WriteWAVTemp(samples []int16, sampleRate, channels int) (string, error) {
	f, err := os.CreateTemp("", "scrib-chunk-*.wav")
	if err != nil {
		return "", err
	}
	path := f.Name()
	f.Close()
	if err := WriteWAV(path, samples, sampleRate, channels); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// ReadWAVMono reads a 16-bit PCM mono WAV file and returns its samples.
// Used by the retry path where the uploader is restarted against an on-disk
// mono WAV already prepared during phaseProcessing.
func ReadWAVMono(path string) ([]int16, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header := make([]byte, 44)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, fmt.Errorf("read wav header: %w", err)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a WAV file: %s", path)
	}
	channels := binary.LittleEndian.Uint16(header[22:24])
	bitsPerSample := binary.LittleEndian.Uint16(header[34:36])
	if channels != 1 || bitsPerSample != 16 {
		return nil, fmt.Errorf("expected 16-bit mono WAV, got %d-bit %d-channel", bitsPerSample, channels)
	}
	dataSize := binary.LittleEndian.Uint32(header[40:44])
	samples := make([]int16, dataSize/2)
	if err := binary.Read(f, binary.LittleEndian, &samples); err != nil {
		return nil, fmt.Errorf("read samples: %w", err)
	}
	return samples, nil
}
