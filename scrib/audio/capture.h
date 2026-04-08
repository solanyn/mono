#ifndef CAPTURE_H
#define CAPTURE_H

#include <stdint.h>

// Start capturing system audio (ScreenCaptureKit) and mic (CoreAudio).
// sample_rate: desired sample rate (e.g. 16000).
// Returns 0 on success.
int start_capture(int sample_rate);

// Stop all capture streams.
void stop_capture(void);

// Callback into Go — implemented in recorder.go
extern void goAudioCallback(int16_t *data, int frameCount, int channels, int isMic);

#endif
