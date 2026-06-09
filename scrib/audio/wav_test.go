package audio

import (
	"math"
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

func TestHighPassEmpty(t *testing.T) {
	out := HighPass(nil, 16000, 80)
	if len(out) != 0 {
		t.Errorf("expected empty, got %d", len(out))
	}
}

func TestHighPassRemovesLowFrequency(t *testing.T) {
	sr := 16000
	dur := 2.0
	n := int(dur * float64(sr))
	samples := make([]int16, n)
	for i := range samples {
		samples[i] = int16(10000 * math.Sin(2*math.Pi*30*float64(i)/float64(sr)))
	}

	out := HighPass(samples, sr, 80)

	var sumSq float64
	for _, s := range out[sr/2:] {
		sumSq += float64(s) * float64(s)
	}
	rmsOut := math.Sqrt(sumSq / float64(len(out[sr/2:])))

	var sumSqIn float64
	for _, s := range samples[sr/2:] {
		sumSqIn += float64(s) * float64(s)
	}
	rmsIn := math.Sqrt(sumSqIn / float64(len(samples[sr/2:])))

	attenuation := rmsOut / rmsIn
	if attenuation > 0.15 {
		t.Errorf("30Hz tone should be attenuated >85%%, got %.1f%% remaining", attenuation*100)
	}
}

func TestHighPassPreservesSpeech(t *testing.T) {
	sr := 16000
	dur := 1.0
	n := int(dur * float64(sr))
	samples := make([]int16, n)
	for i := range samples {
		samples[i] = int16(10000 * math.Sin(2*math.Pi*500*float64(i)/float64(sr)))
	}

	out := HighPass(samples, sr, 80)

	var sumSqOut float64
	for _, s := range out[sr/4:] {
		sumSqOut += float64(s) * float64(s)
	}
	rmsOut := math.Sqrt(sumSqOut / float64(len(out[sr/4:])))

	var sumSqIn float64
	for _, s := range samples[sr/4:] {
		sumSqIn += float64(s) * float64(s)
	}
	rmsIn := math.Sqrt(sumSqIn / float64(len(samples[sr/4:])))

	ratio := rmsOut / rmsIn
	if ratio < 0.95 {
		t.Errorf("500Hz tone should pass through with >95%% energy, got %.1f%%", ratio*100)
	}
}

func TestHighPassRemovesDCOffset(t *testing.T) {
	sr := 16000
	n := sr
	samples := make([]int16, n)
	for i := range samples {
		samples[i] = 5000
	}

	out := HighPass(samples, sr, 80)

	var sum float64
	for _, s := range out[sr/2:] {
		sum += float64(s)
	}
	avg := sum / float64(len(out[sr/2:]))
	if math.Abs(avg) > 50 {
		t.Errorf("DC offset should be removed, got average %.1f", avg)
	}
}

func TestHighPassOutputLength(t *testing.T) {
	samples := make([]int16, 1000)
	out := HighPass(samples, 16000, 80)
	if len(out) != len(samples) {
		t.Errorf("output length %d != input length %d", len(out), len(samples))
	}
}

func TestHighPassNoClipping(t *testing.T) {
	sr := 16000
	n := sr
	samples := make([]int16, n)
	for i := range samples {
		samples[i] = 32000
	}

	out := HighPass(samples, sr, 80)
	for i, s := range out {
		if s > 32767 || s < -32768 {
			t.Errorf("sample %d clipped: %d", i, s)
			break
		}
	}
}
