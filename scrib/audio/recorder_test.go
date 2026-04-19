package audio

import (
	"math"
	"sync"
	"testing"
)

func TestInitialLevelsZero(t *testing.T) {
	r := &Recorder{sampleRate: 16000}
	if r.MicLevel() != 0.0 {
		t.Errorf("initial MicLevel = %f, want 0.0", r.MicLevel())
	}
	if r.SysLevel() != 0.0 {
		t.Errorf("initial SysLevel = %f, want 0.0", r.SysLevel())
	}
}

func TestExponentialSmoothing(t *testing.T) {
	r := &Recorder{sampleRate: 16000}
	r.mu.Lock()
	r.recording = true
	r.mu.Unlock()

	amplitude := int16(16384)
	frames := 1600
	buf := make([]int16, frames)
	for i := range buf {
		buf[i] = amplitude
	}

	r.onAudioGo(buf, frames, 1, true, 0.0)

	first := r.MicLevel()
	if first <= 0 {
		t.Fatalf("MicLevel after first chunk = %f, want > 0", first)
	}

	expectedRMS := float64(amplitude) / 32768.0
	expectedSmoothed := 0.3 * expectedRMS
	if math.Abs(first-expectedSmoothed) > 0.01 {
		t.Errorf("MicLevel = %f, want ~%f", first, expectedSmoothed)
	}

	silent := make([]int16, frames)
	r.onAudioGo(silent, frames, 1, true, 0.1)

	decayed := r.MicLevel()
	if decayed >= first {
		t.Errorf("level should decay: got %f, previous %f", decayed, first)
	}
	expectedDecay := 0.7 * first
	if math.Abs(decayed-expectedDecay) > 0.01 {
		t.Errorf("decayed level = %f, want ~%f", decayed, expectedDecay)
	}
}

func TestLevelsConcurrentAccess(t *testing.T) {
	r := &Recorder{sampleRate: 16000}
	r.mu.Lock()
	r.recording = true
	r.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = r.MicLevel()
		}()
		go func() {
			defer wg.Done()
			_ = r.SysLevel()
		}()
	}

	buf := make([]int16, 160)
	for i := range buf {
		buf[i] = 1000
	}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			r.onAudioGo(buf, 160, 1, true, float64(i)*0.01)
		}
		close(done)
	}()

	wg.Wait()
	<-done
}

func TestTimestampAlignment(t *testing.T) {
	r := &Recorder{sampleRate: 16000}
	r.mu.Lock()
	r.recording = true
	r.mu.Unlock()

	mic := make([]int16, 160)
	for i := range mic {
		mic[i] = 1000
	}
	sys := make([]int16, 160)
	for i := range sys {
		sys[i] = 2000
	}

	r.onAudioGo(mic, 160, 1, true, 1.0)
	r.onAudioGo(sys, 160, 1, false, 1.0)

	r.mu.Lock()
	out := r.interleave()
	r.mu.Unlock()

	if len(out) != 160*2 {
		t.Fatalf("expected %d samples, got %d", 160*2, len(out))
	}

	for i := 0; i < 160; i++ {
		if out[i*2] != 1000 {
			t.Errorf("frame %d L: got %d, want 1000", i, out[i*2])
			break
		}
		if out[i*2+1] != 2000 {
			t.Errorf("frame %d R: got %d, want 2000", i, out[i*2+1])
			break
		}
	}
}

func TestTimestampAlignmentWithOffset(t *testing.T) {
	r := &Recorder{sampleRate: 16000}
	r.mu.Lock()
	r.recording = true
	r.mu.Unlock()

	mic := make([]int16, 160)
	for i := range mic {
		mic[i] = 1000
	}
	sys := make([]int16, 160)
	for i := range sys {
		sys[i] = 2000
	}

	r.onAudioGo(mic, 160, 1, true, 1.0)
	r.onAudioGo(sys, 160, 1, false, 1.005)

	r.mu.Lock()
	out := r.interleave()
	r.mu.Unlock()

	offsetFrames := int(math.Round(0.005 * 16000))

	totalFrames := 160 + offsetFrames
	if len(out) != totalFrames*2 {
		t.Fatalf("expected %d samples, got %d", totalFrames*2, len(out))
	}

	if out[0] != 1000 {
		t.Errorf("first mic sample: got %d, want 1000", out[0])
	}
	if out[1] != 0 {
		t.Errorf("first sys sample should be zero-padded: got %d, want 0", out[1])
	}

	sysStartIdx := offsetFrames*2 + 1
	if out[sysStartIdx] != 2000 {
		t.Errorf("sys at offset %d: got %d, want 2000", offsetFrames, out[sysStartIdx])
	}
}

func TestDriftDetection(t *testing.T) {
	r := &Recorder{sampleRate: 16000}
	r.mu.Lock()
	r.recording = true
	r.mu.Unlock()

	mic := make([]int16, 16000)
	for i := range mic {
		mic[i] = 1000
	}
	sys := make([]int16, 16000)
	for i := range sys {
		sys[i] = 2000
	}

	r.onAudioGo(mic, 16000, 1, true, 0.0)
	r.onAudioGo(sys, 16000, 1, false, 0.1)

	r.mu.Lock()
	out := r.interleave()
	r.mu.Unlock()

	if out == nil {
		t.Fatal("interleave returned nil")
	}

	sr := float64(r.sampleRate)
	micEnd := r.micChunks[len(r.micChunks)-1].timestamp +
		float64(len(r.micChunks[len(r.micChunks)-1].samples))/sr
	sysEnd := r.sysChunks[len(r.sysChunks)-1].timestamp +
		float64(len(r.sysChunks[len(r.sysChunks)-1].samples))/sr
	drift := math.Abs(micEnd - sysEnd)
	if drift <= driftWarningThreshold {
		t.Errorf("expected drift > %fms, got %fms", driftWarningThreshold*1000, drift*1000)
	}
}

func TestSingletonDoubleStartPanics(t *testing.T) {
	globalMu.Lock()
	globalRecorder = &Recorder{sampleRate: 16000}
	globalMu.Unlock()

	defer func() {
		globalMu.Lock()
		globalRecorder = nil
		globalMu.Unlock()

		r := recover()
		if r == nil {
			t.Fatal("expected panic on double Start(), got none")
		}
		msg, ok := r.(string)
		if !ok || !contains(msg, "another recorder is active") {
			t.Errorf("unexpected panic message: %v", r)
		}
	}()

	r2 := &Recorder{sampleRate: 16000}
	globalMu.Lock()
	if globalRecorder != nil {
		globalMu.Unlock()
		panic("audio: Start() called while another recorder is active; call Stop() first")
	}
	globalRecorder = r2
	globalMu.Unlock()
}

func TestSingletonStopThenStartOK(t *testing.T) {
	globalMu.Lock()
	globalRecorder = &Recorder{sampleRate: 16000}
	globalMu.Unlock()

	globalMu.Lock()
	globalRecorder = nil
	globalMu.Unlock()

	globalMu.Lock()
	if globalRecorder != nil {
		globalMu.Unlock()
		t.Fatal("expected globalRecorder to be nil after stop")
	}
	globalRecorder = &Recorder{sampleRate: 16000}
	globalMu.Unlock()

	globalMu.Lock()
	globalRecorder = nil
	globalMu.Unlock()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestZeroPadLaggingChannel(t *testing.T) {
	r := &Recorder{sampleRate: 16000}
	r.mu.Lock()
	r.recording = true
	r.mu.Unlock()

	mic := make([]int16, 320)
	for i := range mic {
		mic[i] = 1000
	}
	sys := make([]int16, 160)
	for i := range sys {
		sys[i] = 2000
	}

	r.onAudioGo(mic, 320, 1, true, 0.0)
	r.onAudioGo(sys, 160, 1, false, 0.0)

	r.mu.Lock()
	out := r.interleave()
	r.mu.Unlock()

	if len(out) != 320*2 {
		t.Fatalf("expected %d samples, got %d", 320*2, len(out))
	}

	for i := 160; i < 320; i++ {
		if out[i*2+1] != 0 {
			t.Errorf("frame %d R should be zero-padded: got %d", i, out[i*2+1])
			break
		}
	}

	for i := 0; i < 160; i++ {
		if out[i*2+1] != 2000 {
			t.Errorf("frame %d R: got %d, want 2000", i, out[i*2+1])
			break
		}
	}
}
