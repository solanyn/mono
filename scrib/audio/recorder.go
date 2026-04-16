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
	"math"
	"sync"
	"unsafe"
)

type Recorder struct {
	sampleRate int
	mu         sync.Mutex
	samples    []int16
	recording  bool

	levelMu  sync.Mutex
	micLevel float64
	sysLevel float64
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
	src := unsafe.Slice((*int16)(unsafe.Pointer(data)), frameCount*channels)
	r.onAudioGo(src, frameCount, channels, isMic)
}

func (r *Recorder) onAudioGo(src []int16, frameCount, channels int, isMic bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.recording {
		return
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

	// Stereo interleaving layout:
	// Even indices (0, 2, 4, ...) = left channel (mic)
	// Odd indices  (1, 3, 5, ...) = right channel (system audio)
	currentFrames := len(r.samples) / 2

	if isMic {
		// Write mic samples into left channel (even indices)
		for i := 0; i < frameCount; i++ {
			idx := (currentFrames + i) * 2
			for idx+1 >= len(r.samples) {
				r.samples = append(r.samples, 0, 0)
			}
			r.samples[idx] = mono[i]
		}
	} else {
		// Write system audio into right channel (odd indices)
		for i := 0; i < frameCount; i++ {
			idx := (currentFrames+i)*2 + 1
			for idx >= len(r.samples) {
				r.samples = append(r.samples, 0, 0)
			}
			r.samples[idx] = mono[i]
		}
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
	globalRecorder = r
	r.mu.Lock()
	r.recording = true
	r.samples = make([]int16, 0, r.sampleRate*2*60)
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

func (r *Recorder) FrameCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.samples) / 2
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
