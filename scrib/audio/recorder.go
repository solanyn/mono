package audio

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ScreenCaptureKit -framework CoreMedia -framework CoreAudio -framework AudioToolbox -framework Foundation -framework AVFoundation

#include "capture.h"
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

// Recorder captures system audio output via ScreenCaptureKit
// and microphone input via CoreAudio, interleaving into stereo
// (L=mic, R=system).
type Recorder struct {
	sampleRate int
	mu         sync.Mutex
	samples    []int16 // interleaved stereo: L=mic, R=system
	recording  bool
}

func NewRecorder(sampleRate int) (*Recorder, error) {
	return &Recorder{sampleRate: sampleRate}, nil
}

//export goAudioCallback
func goAudioCallback(data *C.int16_t, frameCount C.int, channels C.int, isMic C.int) {
	globalRecorder.onAudio(data, int(frameCount), int(channels), int(isMic) != 0)
}

var globalRecorder *Recorder

func (r *Recorder) onAudio(data *C.int16_t, frameCount, channels int, isMic bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.recording {
		return
	}

	src := unsafe.Slice((*int16)(unsafe.Pointer(data)), frameCount*channels)

	// Mix down to mono for this source
	mono := make([]int16, frameCount)
	for i := 0; i < frameCount; i++ {
		if channels == 1 {
			mono[i] = src[i]
		} else {
			// Average channels
			var sum int32
			for ch := 0; ch < channels; ch++ {
				sum += int32(src[i*channels+ch])
			}
			mono[i] = int16(sum / int32(channels))
		}
	}

	// Ensure samples buffer is large enough, then interleave
	// Stereo layout: even indices = mic (L), odd indices = system (R)
	// We grow the buffer as needed since mic and system callbacks
	// may arrive at different rates
	currentFrames := len(r.samples) / 2

	if isMic {
		// Write to left channel
		for i := 0; i < frameCount; i++ {
			idx := (currentFrames + i) * 2 // left channel
			for idx+1 >= len(r.samples) {
				r.samples = append(r.samples, 0, 0)
			}
			r.samples[idx] = mono[i]
		}
	} else {
		// Write to right channel
		for i := 0; i < frameCount; i++ {
			idx := (currentFrames+i)*2 + 1 // right channel
			for idx >= len(r.samples) {
				r.samples = append(r.samples, 0, 0)
			}
			r.samples[idx] = mono[i]
		}
	}
}

func (r *Recorder) Start() error {
	globalRecorder = r
	r.mu.Lock()
	r.recording = true
	r.samples = make([]int16, 0, r.sampleRate*2*60) // pre-alloc ~1 min stereo
	r.mu.Unlock()

	ret := C.start_capture(C.int(r.sampleRate))
	if ret != 0 {
		return fmt.Errorf("start_capture failed: %d", ret)
	}
	return nil
}

func (r *Recorder) Stop() []int16 {
	C.stop_capture()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recording = false
	out := r.samples
	r.samples = nil
	return out
}

// Snapshot returns a copy of samples recorded so far without stopping.
// Returns samples from offset onwards (interleaved stereo).
func (r *Recorder) Snapshot(fromFrame int) []int16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.recording {
		return nil
	}
	fromIdx := fromFrame * 2
	if fromIdx >= len(r.samples) {
		return nil
	}
	out := make([]int16, len(r.samples)-fromIdx)
	copy(out, r.samples[fromIdx:])
	return out
}

// FrameCount returns the current number of stereo frames recorded.
func (r *Recorder) FrameCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.samples) / 2
}
