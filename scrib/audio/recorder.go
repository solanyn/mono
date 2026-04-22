package audio

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ScreenCaptureKit -framework CoreMedia -framework CoreAudio -framework AudioToolbox -framework Foundation -framework AVFoundation

#include "capture.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
	"unsafe"
)

type timedChunk struct {
	samples   []int16
	timestamp float64
}

type Recorder struct {
	sampleRate int
	mu         sync.Mutex
	micChunks  []timedChunk
	sysChunks  []timedChunk
	recording  bool
	startTime  time.Time    // wall-clock start for backlog detection
	sysAnchor  float64      // first accepted system audio PTS (after warmup)
	sysReady   bool         // true once backlog flush period is over

	levelMu  sync.Mutex
	micLevel float64
	sysLevel float64
}

func NewRecorder(sampleRate int) (*Recorder, error) {
	return &Recorder{sampleRate: sampleRate}, nil
}

//export goAudioCallback
func goAudioCallback(data *C.int16_t, frameCount C.int, channels C.int, isMic C.int, timestampSecs C.double) {
	globalRecorder.onAudio(data, int(frameCount), int(channels), int(isMic) != 0, float64(timestampSecs))
}

// globalRecorder holds the active Recorder instance. Only one Recorder can be
// active at a time because cgo callbacks (goAudioCallback) use this package-level
// variable to route audio data. Start() panics if called while another recorder
// is already active; Stop() clears the global to allow reuse.
var globalRecorder *Recorder
var globalMu sync.Mutex

func (r *Recorder) onAudio(data *C.int16_t, frameCount, channels int, isMic bool, timestamp float64) {
	src := unsafe.Slice((*int16)(unsafe.Pointer(data)), frameCount*channels)
	r.onAudioGo(src, frameCount, channels, isMic, timestamp)
}

func (r *Recorder) onAudioGo(src []int16, frameCount, channels int, isMic bool, timestamp float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.recording {
		return
	}

	// Discard stale system audio backlog from ScreenCaptureKit.
	// SCK delivers buffered audio from before recording started — the
	// C layer normalizes PTS relative to the first callback, so backlog
	// spans 0..N seconds delivered in milliseconds. We flush for 1s of
	// wall-clock time, then re-anchor PTS to the first real chunk.
	if !isMic {
		elapsed := time.Since(r.startTime).Seconds()
		if !r.sysReady {
			if elapsed < 1.0 {
				// Still in warmup — discard backlog
				return
			}
			// Warmup done — anchor to this chunk's PTS
			r.sysReady = true
			r.sysAnchor = timestamp
			log.Printf("[scrib] system audio anchored at PTS=%.3fs (wall=%.3fs), backlog discarded", timestamp, elapsed)
		}
		// Re-base system audio timestamps to match wall-clock
		timestamp = timestamp - r.sysAnchor + 1.0 // +1.0 accounts for warmup period
	}

	mono := make([]int16, frameCount)
	var sumSq float64
	for i := 0; i < frameCount; i++ {
		if channels == 1 {
			mono[i] = src[i]
		} else {
			var sum int32
			for ch := 0; ch < channels; ch++ {
				sum += int32(src[i*channels+ch])
			}
			mono[i] = int16(sum / int32(channels))
		}
		s := float64(mono[i]) / 32768.0
		sumSq += s * s
	}

	rms := math.Sqrt(sumSq / float64(frameCount))
	r.levelMu.Lock()
	if isMic {
		r.micLevel = 0.3*rms + 0.7*r.micLevel
	} else {
		r.sysLevel = 0.3*rms + 0.7*r.sysLevel
	}
	r.levelMu.Unlock()

	chunk := timedChunk{samples: mono, timestamp: timestamp}
	if isMic {
		r.micChunks = append(r.micChunks, chunk)
	} else {
		r.sysChunks = append(r.sysChunks, chunk)
	}
}

const driftWarningThreshold = 0.050

func (r *Recorder) interleave() []int16 {
	if len(r.micChunks) == 0 && len(r.sysChunks) == 0 {
		return nil
	}

	sr := float64(r.sampleRate)

	t0 := math.Inf(1)
	tEnd := math.Inf(-1)
	for _, c := range r.micChunks {
		if c.timestamp < t0 {
			t0 = c.timestamp
		}
		end := c.timestamp + float64(len(c.samples))/sr
		if end > tEnd {
			tEnd = end
		}
	}
	for _, c := range r.sysChunks {
		if c.timestamp < t0 {
			t0 = c.timestamp
		}
		end := c.timestamp + float64(len(c.samples))/sr
		if end > tEnd {
			tEnd = end
		}
	}

	totalFrames := int(math.Round((tEnd - t0) * sr))
	if totalFrames <= 0 {
		return nil
	}

	out := make([]int16, totalFrames*2)

	for _, c := range r.micChunks {
		offset := int(math.Round((c.timestamp - t0) * sr))
		for i, s := range c.samples {
			idx := (offset + i) * 2
			if idx >= 0 && idx < len(out) {
				out[idx] = s
			}
		}
	}

	for _, c := range r.sysChunks {
		offset := int(math.Round((c.timestamp - t0) * sr))
		for i, s := range c.samples {
			idx := (offset+i)*2 + 1
			if idx >= 0 && idx < len(out) {
				out[idx] = s
			}
		}
	}

	r.checkDrift()

	return out
}

func (r *Recorder) checkDrift() {
	if len(r.micChunks) == 0 || len(r.sysChunks) == 0 {
		return
	}

	sr := float64(r.sampleRate)

	micEnd := r.micChunks[len(r.micChunks)-1].timestamp +
		float64(len(r.micChunks[len(r.micChunks)-1].samples))/sr
	sysEnd := r.sysChunks[len(r.sysChunks)-1].timestamp +
		float64(len(r.sysChunks[len(r.sysChunks)-1].samples))/sr

	drift := math.Abs(micEnd - sysEnd)
	if drift > driftWarningThreshold {
		log.Printf("[scrib] audio channel drift detected: %.1fms (mic end=%.3fs, sys end=%.3fs)",
			drift*1000, micEnd, sysEnd)
	}
}

func (r *Recorder) MicLevel() float64 {
	r.levelMu.Lock()
	defer r.levelMu.Unlock()
	return r.micLevel
}

func (r *Recorder) SysLevel() float64 {
	r.levelMu.Lock()
	defer r.levelMu.Unlock()
	return r.sysLevel
}

func (r *Recorder) Start() error {
	globalMu.Lock()
	if globalRecorder != nil {
		globalMu.Unlock()
		panic("audio: Start() called while another recorder is active; call Stop() first")
	}
	globalRecorder = r
	globalMu.Unlock()

	r.mu.Lock()
	r.recording = true
	r.startTime = time.Now()
	r.sysAnchor = 0
	r.sysReady = false
	r.micChunks = nil
	r.sysChunks = nil
	r.mu.Unlock()

	ret := C.start_capture(C.int(r.sampleRate))
	if ret != 0 {
		globalMu.Lock()
		globalRecorder = nil
		globalMu.Unlock()
		return fmt.Errorf("start_capture failed: %d", ret)
	}
	return nil
}

func (r *Recorder) Stop() []int16 {
	C.stop_capture()
	r.mu.Lock()
	r.recording = false
	out := r.interleave()
	r.micChunks = nil
	r.sysChunks = nil
	r.mu.Unlock()

	globalMu.Lock()
	globalRecorder = nil
	globalMu.Unlock()

	return out
}

func (r *Recorder) Snapshot(fromFrame int) []int16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.recording {
		return nil
	}
	all := r.interleave()
	fromIdx := fromFrame * 2
	if fromIdx >= len(all) {
		return nil
	}
	out := make([]int16, len(all)-fromIdx)
	copy(out, all[fromIdx:])
	return out
}

func (r *Recorder) FrameCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	total := 0
	for _, c := range r.micChunks {
		total += len(c.samples)
	}
	for _, c := range r.sysChunks {
		total += len(c.samples)
	}
	if total == 0 {
		return 0
	}
	sr := float64(r.sampleRate)
	t0 := math.Inf(1)
	tEnd := math.Inf(-1)
	for _, c := range r.micChunks {
		if c.timestamp < t0 {
			t0 = c.timestamp
		}
		end := c.timestamp + float64(len(c.samples))/sr
		if end > tEnd {
			tEnd = end
		}
	}
	for _, c := range r.sysChunks {
		if c.timestamp < t0 {
			t0 = c.timestamp
		}
		end := c.timestamp + float64(len(c.samples))/sr
		if end > tEnd {
			tEnd = end
		}
	}
	return int(math.Round((tEnd - t0) * sr))
}

func GetInputDeviceName() string {
	cstr := C.get_input_device_name()
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr)
}

func GetOutputDeviceName() string {
	cstr := C.get_output_device_name()
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr)
}

func GetCaptureStatus() int {
	return int(C.get_capture_status())
}
