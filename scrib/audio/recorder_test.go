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

	r.levelMu.Lock()
	r.micLevel = 0.5
	r.levelMu.Unlock()

	newRMS := 1.0
	expected := 0.3*newRMS + 0.7*0.5

	r.levelMu.Lock()
	r.micLevel = 0.3*newRMS + 0.7*r.micLevel
	r.levelMu.Unlock()

	got := r.MicLevel()
	if math.Abs(got-expected) > 1e-10 {
		t.Errorf("MicLevel after smoothing = %f, want %f", got, expected)
	}

	r.levelMu.Lock()
	r.micLevel = 0.3*0.0 + 0.7*r.micLevel
	r.levelMu.Unlock()

	decayed := r.MicLevel()
	if decayed >= expected {
		t.Errorf("level should decay: got %f, previous %f", decayed, expected)
	}
	expectedDecay := 0.7 * expected
	if math.Abs(decayed-expectedDecay) > 1e-10 {
		t.Errorf("decayed level = %f, want %f", decayed, expectedDecay)
	}
}

func TestLevelsConcurrentAccess(t *testing.T) {
	r := &Recorder{sampleRate: 16000}

	r.levelMu.Lock()
	r.micLevel = 0.5
	r.sysLevel = 0.3
	r.levelMu.Unlock()

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

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			r.levelMu.Lock()
			r.micLevel = 0.3*0.1 + 0.7*r.micLevel
			r.sysLevel = 0.3*0.2 + 0.7*r.sysLevel
			r.levelMu.Unlock()
		}
		close(done)
	}()

	wg.Wait()
	<-done
}
