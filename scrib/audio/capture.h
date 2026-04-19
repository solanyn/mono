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
// timestampSecs is the presentation timestamp in seconds (host time).
extern void goAudioCallback(int16_t *data, int frameCount, int channels, int isMic, double timestampSecs);

// Returns the name of the default input (mic) device. Caller must free().
const char* get_input_device_name(void);

// Returns the name of the default output (speakers) device. Caller must free().
const char* get_output_device_name(void);

// Capture status: 0=both streams ok, 1=mic-only (system audio failed), -1=failed
int get_capture_status(void);

#endif
