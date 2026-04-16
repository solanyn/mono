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
	r.samples = make([]int16, 0, 16000*2)
	r.mu.Unlock()

	amplitude := int16(16384)
	frames := 1600
	buf := make([]int16, frames)
	for i := range buf {
		buf[i] = amplitude
	}

	r.onAudioGo(buf, frames, 1, true)

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
	r.onAudioGo(silent, frames, 1, true)

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
	r.samples = make([]int16, 0, 16000*2)
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
			r.onAudioGo(buf, 160, 1, true)
		}
		close(done)
	}()

	wg.Wait()
	<-done
}
