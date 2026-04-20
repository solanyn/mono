package audio

import (
	"os"
	"testing"
)

func TestStereoToMono(t *testing.T) {
	stereo := []int16{100, 200, 300, 400, -100, -200}
	mono := StereoToMono(stereo)
	if len(mono) != 3 {
		t.Fatalf("expected 3 frames, got %d", len(mono))
	}
	want := []int16{150, 350, -150}
	for i, v := range want {
		if mono[i] != v {
			t.Errorf("frame %d: got %d, want %d", i, mono[i], v)
		}
	}
}

func TestStereoToMonoEmpty(t *testing.T) {
	mono := StereoToMono(nil)
	if len(mono) != 0 {
		t.Errorf("expected empty, got %d", len(mono))
	}
}

func TestWriteWAVRoundTrip(t *testing.T) {
	samples := []int16{1000, 2000, 3000, 4000}
	tmp, err := WriteWAVTemp(samples, 16000, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp)

	info, err := os.Stat(tmp)
	if err != nil {
		t.Fatal(err)
	}
	expectedSize := 44 + len(samples)*2
	if info.Size() != int64(expectedSize) {
		t.Errorf("file size = %d, want %d", info.Size(), expectedSize)
	}
}

func TestStereoToMonoThenWriteWAV(t *testing.T) {
	stereo := []int16{100, 200, 300, 400}
	mono := StereoToMono(stereo)
	tmp, err := WriteWAVTemp(mono, 16000, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp)

	info, _ := os.Stat(tmp)
	expectedSize := 44 + len(mono)*2
	if info.Size() != int64(expectedSize) {
		t.Errorf("mono WAV size = %d, want %d", info.Size(), expectedSize)
	}
}
