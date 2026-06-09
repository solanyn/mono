package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
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

func HighPass(samples []int16, sampleRate int, cutoffHz float64) []int16 {
	if len(samples) == 0 {
		return samples
	}
	omega := 2.0 * math.Pi * cutoffHz / float64(sampleRate)
	alpha := math.Sin(omega) / (2.0 * 0.7071)

	b0 := (1.0 + math.Cos(omega)) / 2.0
	b1 := -(1.0 + math.Cos(omega))
	b2 := (1.0 + math.Cos(omega)) / 2.0
	a0 := 1.0 + alpha
	a1 := -2.0 * math.Cos(omega)
	a2 := 1.0 - alpha

	b0 /= a0
	b1 /= a0
	b2 /= a0
	a1 /= a0
	a2 /= a0

	out := make([]int16, len(samples))
	var x1, x2, y1, y2 float64
	for i, s := range samples {
		x0 := float64(s)
		y0 := b0*x0 + b1*x1 + b2*x2 - a1*y1 - a2*y2
		if y0 > 32767 {
			y0 = 32767
		} else if y0 < -32768 {
			y0 = -32768
		}
		out[i] = int16(y0)
		x2 = x1
		x1 = x0
		y2 = y1
		y1 = y0
	}
	return out
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

// ReadWAV reads a 16-bit PCM WAV file (mono or stereo) and returns its
// interleaved samples along with the channel count.
func ReadWAV(path string) (samples []int16, channels int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	header := make([]byte, 44)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, 0, fmt.Errorf("read wav header: %w", err)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("not a WAV file: %s", path)
	}
	ch := binary.LittleEndian.Uint16(header[22:24])
	bitsPerSample := binary.LittleEndian.Uint16(header[34:36])
	if bitsPerSample != 16 {
		return nil, 0, fmt.Errorf("expected 16-bit WAV, got %d-bit", bitsPerSample)
	}
	if ch != 1 && ch != 2 {
		return nil, 0, fmt.Errorf("expected 1 or 2 channels, got %d", ch)
	}
	dataSize := binary.LittleEndian.Uint32(header[40:44])
	samples = make([]int16, dataSize/2)
	if err := binary.Read(f, binary.LittleEndian, &samples); err != nil {
		return nil, 0, fmt.Errorf("read samples: %w", err)
	}
	return samples, int(ch), nil
}

// ReadWAVMono reads a 16-bit PCM mono WAV file and returns its samples.
func ReadWAVMono(path string) ([]int16, error) {
	samples, ch, err := ReadWAV(path)
	if err != nil {
		return nil, err
	}
	if ch != 1 {
		return nil, fmt.Errorf("expected 16-bit mono WAV, got 16-bit %d-channel", ch)
	}
	return samples, nil
}
