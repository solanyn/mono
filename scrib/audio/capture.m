// capture.m — ScreenCaptureKit system audio + CoreAudio mic capture
// macOS 13+ required

#import <Foundation/Foundation.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <CoreMedia/CoreMedia.h>
#import <AudioToolbox/AudioToolbox.h>
#import <AVFoundation/AVFoundation.h>
#include "capture.h"
#include <stdatomic.h>

// ─── System Audio (ScreenCaptureKit) ───────────────────────────────

static _Atomic double sysAudioStartTime = -1.0;

@interface SystemAudioDelegate : NSObject <SCStreamOutput, SCStreamDelegate>
@end

@implementation SystemAudioDelegate

- (void)stream:(SCStream *)stream
    didOutputSampleBuffer:(CMSampleBufferRef)sampleBuffer
               ofType:(SCStreamOutputType)type {
    if (type != SCStreamOutputTypeAudio) return;

    CMBlockBufferRef blockBuffer = CMSampleBufferGetDataBuffer(sampleBuffer);
    if (!blockBuffer) return;

    size_t totalLen = 0;
    char *dataPtr = NULL;
    OSStatus status = CMBlockBufferGetDataPointer(blockBuffer, 0, NULL, &totalLen, &dataPtr);
    if (status != noErr || !dataPtr) return;

    // Get audio format
    CMFormatDescriptionRef fmt = CMSampleBufferGetFormatDescription(sampleBuffer);
    const AudioStreamBasicDescription *asbd = CMAudioFormatDescriptionGetStreamBasicDescription(fmt);
    if (!asbd) return;

    // ScreenCaptureKit delivers float32 interleaved
    int channels = (int)asbd->mChannelsPerFrame;
    int frameCount = (int)(totalLen / (sizeof(float) * channels));

    // Convert float32 → int16
    float *floatData = (float *)dataPtr;
    int16_t *pcm = (int16_t *)malloc(frameCount * channels * sizeof(int16_t));
    if (!pcm) return;

    for (int i = 0; i < frameCount * channels; i++) {
        float sample = floatData[i];
        if (sample > 1.0f) sample = 1.0f;
        if (sample < -1.0f) sample = -1.0f;
        pcm[i] = (int16_t)(sample * 32767.0f);
    }

    CMTime pts = CMSampleBufferGetPresentationTimeStamp(sampleBuffer);
    double timestampSecs = CMTimeGetSeconds(pts);

    double expected = __c11_atomic_load(&sysAudioStartTime, __ATOMIC_RELAXED);
    if (expected < 0.0) {
        __c11_atomic_store(&sysAudioStartTime, timestampSecs, __ATOMIC_RELAXED);
        timestampSecs = 0.0;
    } else {
        timestampSecs -= expected;
    }

    goAudioCallback(pcm, frameCount, channels, 0, timestampSecs);
    free(pcm);
}

- (void)stream:(SCStream *)stream didStopWithError:(NSError *)error {
    if (error) {
        NSLog(@"[meet] SCStream stopped with error: %@", error);
    }
}

@end

// ─── Microphone (CoreAudio via AudioQueue) ─────────────────────────

static AudioQueueRef micQueue = NULL;
static int gSampleRate = 16000;

static void micCallback(void *userData,
                         AudioQueueRef queue,
                         AudioQueueBufferRef buffer,
                         const AudioTimeStamp *startTime,
                         UInt32 numPackets,
                         const AudioStreamPacketDescription *packetDesc) {
    if (numPackets == 0) return;

    int16_t *data = (int16_t *)buffer->mAudioData;
    int frameCount = (int)(buffer->mAudioDataByteSize / sizeof(int16_t));

    double timestampSecs = startTime->mSampleTime / (double)gSampleRate;

    goAudioCallback(data, frameCount, 1, 1, timestampSecs);

    // Re-enqueue buffer
    AudioQueueEnqueueBuffer(queue, buffer, 0, NULL);
}

static int start_mic(int sampleRate) {
    AudioStreamBasicDescription fmt = {0};
    fmt.mSampleRate = (Float64)sampleRate;
    fmt.mFormatID = kAudioFormatLinearPCM;
    fmt.mFormatFlags = kLinearPCMFormatFlagIsSignedInteger | kLinearPCMFormatFlagIsPacked;
    fmt.mBitsPerChannel = 16;
    fmt.mChannelsPerFrame = 1;
    fmt.mBytesPerFrame = 2;
    fmt.mFramesPerPacket = 1;
    fmt.mBytesPerPacket = 2;

    OSStatus status = AudioQueueNewInput(&fmt, micCallback, NULL, NULL, NULL, 0, &micQueue);
    if (status != noErr) {
        NSLog(@"[meet] AudioQueueNewInput failed: %d", (int)status);
        return -1;
    }

    // Allocate 3 buffers (~100ms each)
    int bufferSize = sampleRate / 10 * 2; // 100ms of int16 mono
    for (int i = 0; i < 3; i++) {
        AudioQueueBufferRef buf;
        AudioQueueAllocateBuffer(micQueue, bufferSize, &buf);
        AudioQueueEnqueueBuffer(micQueue, buf, 0, NULL);
    }

    status = AudioQueueStart(micQueue, NULL);
    if (status != noErr) {
        NSLog(@"[meet] AudioQueueStart (mic) failed: %d", (int)status);
        return -2;
    }

    return 0;
}

static void stop_mic(void) {
    if (micQueue) {
        AudioQueueStop(micQueue, true);
        AudioQueueDispose(micQueue, true);
        micQueue = NULL;
    }
}

// ─── ScreenCaptureKit system audio ─────────────────────────────────

static SCStream *scStream = NULL;
static SystemAudioDelegate *scDelegate = NULL;

static int start_system_audio(int sampleRate) {
    __block int result = 0;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);

    [SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent *content, NSError *error) {
        if (error || !content) {
            NSLog(@"[meet] getShareableContent failed: %@", error);
            result = -1;
            dispatch_semaphore_signal(sem);
            return;
        }

        // We need a display for the content filter, but we only want audio
        if (content.displays.count == 0) {
            NSLog(@"[meet] No displays found");
            result = -2;
            dispatch_semaphore_signal(sem);
            return;
        }

        SCDisplay *display = content.displays[0];

        // Create filter that excludes all windows — we only want audio
        SCContentFilter *filter = [[SCContentFilter alloc]
            initWithDisplay:display
            excludingWindows:@[]];

        SCStreamConfiguration *config = [[SCStreamConfiguration alloc] init];
        config.capturesAudio = YES;
        config.excludesCurrentProcessAudio = NO;
        config.channelCount = 2;
        config.sampleRate = sampleRate;

        // Disable video capture — audio only
        config.width = 2;
        config.height = 2;
        config.minimumFrameInterval = CMTimeMake(1, 1); // 1 fps minimum
        config.showsCursor = NO;

        scDelegate = [[SystemAudioDelegate alloc] init];
        scStream = [[SCStream alloc] initWithFilter:filter configuration:config delegate:scDelegate];

        NSError *addErr = nil;
        [scStream addStreamOutput:scDelegate type:SCStreamOutputTypeAudio sampleHandlerQueue:dispatch_get_global_queue(QOS_CLASS_USER_INTERACTIVE, 0) error:&addErr];
        if (addErr) {
            NSLog(@"[meet] addStreamOutput failed: %@", addErr);
            result = -3;
            dispatch_semaphore_signal(sem);
            return;
        }

        [scStream startCaptureWithCompletionHandler:^(NSError *startErr) {
            if (startErr) {
                NSLog(@"[meet] startCapture failed: %@", startErr);
                result = -4;
            }
            dispatch_semaphore_signal(sem);
        }];
    }];

    dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));
    return result;
}

static void stop_system_audio(void) {
    if (scStream) {
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        [scStream stopCaptureWithCompletionHandler:^(NSError *error) {
            dispatch_semaphore_signal(sem);
        }];
        dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC));
        scStream = nil;
        scDelegate = nil;
    }
    __c11_atomic_store(&sysAudioStartTime, -1.0, __ATOMIC_RELAXED);
}

// ─── Public API ────────────────────────────────────────────────────

static _Atomic int gCaptureStatus = 0;

int start_capture(int sample_rate) {
    gSampleRate = sample_rate;

    int sysRet = start_system_audio(sample_rate);
    if (sysRet != 0) {
        NSLog(@"[meet] System audio capture failed (%d), continuing with mic only", sysRet);
    }

    int micRet = start_mic(sample_rate);
    if (micRet != 0) {
        NSLog(@"[meet] Mic capture failed (%d)", micRet);
        stop_system_audio();
        gCaptureStatus = -1;
        return micRet;
    }

    gCaptureStatus = (sysRet != 0) ? 1 : 0;
    return 0;
}

void stop_capture(void) {
    stop_mic();
    stop_system_audio();
}

int get_capture_status(void) {
    return gCaptureStatus;
}

static const char* get_device_name(AudioObjectID deviceID) {
    if (deviceID == kAudioObjectUnknown) return strdup("Unknown");

    AudioObjectPropertyAddress nameAddr = {
        .mSelector = kAudioObjectPropertyName,
        .mScope    = kAudioObjectPropertyScopeGlobal,
        .mElement  = kAudioObjectPropertyElementMain,
    };

    CFStringRef cfName = NULL;
    UInt32 size = sizeof(cfName);
    OSStatus status = AudioObjectGetPropertyData(deviceID, &nameAddr, 0, NULL, &size, &cfName);
    if (status != noErr || !cfName) return strdup("Unknown");

    char buf[256];
    if (!CFStringGetCString(cfName, buf, sizeof(buf), kCFStringEncodingUTF8)) {
        CFRelease(cfName);
        return strdup("Unknown");
    }
    CFRelease(cfName);
    return strdup(buf);
}

const char* get_input_device_name(void) {
    AudioObjectPropertyAddress addr = {
        .mSelector = kAudioHardwarePropertyDefaultInputDevice,
        .mScope    = kAudioObjectPropertyScopeGlobal,
        .mElement  = kAudioObjectPropertyElementMain,
    };
    AudioObjectID deviceID = kAudioObjectUnknown;
    UInt32 size = sizeof(deviceID);
    AudioObjectGetPropertyData(kAudioObjectSystemObject, &addr, 0, NULL, &size, &deviceID);
    return get_device_name(deviceID);
}

const char* get_output_device_name(void) {
    AudioObjectPropertyAddress addr = {
        .mSelector = kAudioHardwarePropertyDefaultOutputDevice,
        .mScope    = kAudioObjectPropertyScopeGlobal,
        .mElement  = kAudioObjectPropertyElementMain,
    };
    AudioObjectID deviceID = kAudioObjectUnknown;
    UInt32 size = sizeof(deviceID);
    AudioObjectGetPropertyData(kAudioObjectSystemObject, &addr, 0, NULL, &size, &deviceID);
    return get_device_name(deviceID);
}
