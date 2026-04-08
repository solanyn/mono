package audio

import (
	"encoding/binary"
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
