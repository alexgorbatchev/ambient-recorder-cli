#import <CoreAudio/CoreAudio.h>
#import <CoreAudio/AudioHardwareTapping.h>
#import <CoreAudio/CATapDescription.h>
#import <AVFoundation/AVFoundation.h>
#import <Foundation/Foundation.h>
#include <stdatomic.h>
#include <math.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <limits.h>
#include "native.h"

_Static_assert(ATOMIC_LLONG_LOCK_FREE == 2, "capture requires lock-free native counters");
enum { AR_MAX_STREAMS = 32, AR_MAX_WATCHES = AR_MAX_STREAMS + 4 };
typedef struct {
    ARChunk info;
    float pcm[AR_MAX_FRAMES];
} ARBlock;
typedef struct {
    AudioObjectID object;
    AudioObjectPropertyAddress address;
} ARWatch;

struct ARCapture {
    AudioObjectID microphone;
    AudioObjectID tap;
    AudioObjectID aggregate;
    AudioDeviceIOProcID io;
    int running;
    double rate;
    uint32_t channels;
    _Atomic uint64_t head;
    _Atomic uint64_t tail;
    _Atomic uint64_t dropped;
    _Atomic int changed;
    ARWatch watches[AR_MAX_WATCHES];
    unsigned watch_count;
    ARBlock blocks[AR_QUEUE_BLOCKS];
};

static OSStatus get_property(AudioObjectID object, AudioObjectPropertySelector selector, AudioObjectPropertyScope scope, UInt32 *size, void *data) {
    AudioObjectPropertyAddress address = {selector, scope, kAudioObjectPropertyElementMain};
    return AudioObjectGetPropertyData(object, &address, 0, NULL, size, data);
}

static char *copy_cf_string(CFStringRef string) {
    CFIndex size = CFStringGetMaximumSizeForEncoding(CFStringGetLength(string), kCFStringEncodingUTF8);
    if (size < 0 || size == LONG_MAX) return NULL;
    size++;
    char *value = malloc((size_t)size);
    if (value && !CFStringGetCString(string, value, size, kCFStringEncodingUTF8)) {
        free(value);
        return NULL;
    }
    return value;
}

char *ar_capture_microphone_name(ARCapture *capture) {
    CFStringRef name = NULL;
    UInt32 size = sizeof(name);
    OSStatus status = get_property(capture->microphone, kAudioObjectPropertyName, kAudioObjectPropertyScopeGlobal, &size, &name);
    if (status != noErr || !name) return NULL;
    char *value = copy_cf_string(name);
    CFRelease(name);
    return value;
}

// The HAL owns these buffers. No allocation, lock, Go callback or I/O is allowed here.
static OSStatus capture_io(AudioObjectID device, const AudioTimeStamp *now, const AudioBufferList *input,
                           const AudioTimeStamp *input_time, AudioBufferList *output,
                           const AudioTimeStamp *output_time, void *user_data) {
    ARCapture *capture = user_data;
    if (atomic_load_explicit(&capture->changed, memory_order_relaxed)) return noErr;
    if (!input || !input->mNumberBuffers) return noErr;
    uint32_t frames = 0, channels = 0;
    for (UInt32 i = 0; i < input->mNumberBuffers; i++) {
        const AudioBuffer *buffer = &input->mBuffers[i];
        if (!buffer->mNumberChannels) continue;
        uint32_t stride = buffer->mNumberChannels * sizeof(float);
        uint32_t n = buffer->mDataByteSize / stride;
        if (buffer->mDataByteSize % stride || (channels && frames != n) || n > AR_MAX_FRAMES) {
            atomic_store_explicit(&capture->changed, 1, memory_order_relaxed);
            return noErr;
        }
        frames = n;
        channels += buffer->mNumberChannels;
    }
    if (!frames) return noErr;
    if (channels != capture->channels) {
        atomic_store_explicit(&capture->changed, 1, memory_order_relaxed);
        return noErr;
    }
    uint64_t head = atomic_load_explicit(&capture->head, memory_order_relaxed);
    uint64_t tail = atomic_load_explicit(&capture->tail, memory_order_acquire);
    if (head - tail >= AR_QUEUE_BLOCKS) {
        atomic_fetch_add_explicit(&capture->dropped, frames, memory_order_relaxed);
        return noErr;
    }
    ARBlock *block = &capture->blocks[head % AR_QUEUE_BLOCKS];
    memset(block->pcm, 0, frames * sizeof(float));
    float scale = 1.f / channels;
    for (UInt32 i = 0; i < input->mNumberBuffers; i++) {
        const AudioBuffer *buffer = &input->mBuffers[i];
        if (!buffer->mData) continue;
        const float *source = buffer->mData;
        for (uint32_t frame = 0; frame < frames; frame++) {
            for (uint32_t channel = 0; channel < buffer->mNumberChannels; channel++) {
                float sample = source[frame * buffer->mNumberChannels + channel];
                if (isfinite(sample)) block->pcm[frame] += sample * scale;
            }
        }
    }
    for (uint32_t i = 0; i < frames; i++) block->pcm[i] = fmaxf(-1.f, fminf(1.f, block->pcm[i]));
    block->info = (ARChunk){frames, input_time->mHostTime, input_time->mSampleTime, input_time->mFlags};
    atomic_store_explicit(&capture->head, head + 1, memory_order_release);
    return noErr;
}

int ar_capture_read(ARCapture *capture, float *pcm, uint32_t capacity, ARChunk *chunk) {
    uint64_t tail = atomic_load_explicit(&capture->tail, memory_order_relaxed);
    uint64_t head = atomic_load_explicit(&capture->head, memory_order_acquire);
    if (tail == head) return 0;
    ARBlock *block = &capture->blocks[tail % AR_QUEUE_BLOCKS];
    if (block->info.frames > capacity) return -1;
    *chunk = block->info;
    memcpy(pcm, block->pcm, chunk->frames * sizeof(float));
    atomic_store_explicit(&capture->tail, tail + 1, memory_order_release);
    return 1;
}

uint64_t ar_capture_dropped(ARCapture *capture) { return atomic_load_explicit(&capture->dropped, memory_order_relaxed); }
int ar_capture_changed(ARCapture *capture) { return atomic_load_explicit(&capture->changed, memory_order_relaxed); }
double ar_capture_rate(ARCapture *capture) { return capture->rate; }
uint32_t ar_capture_microphone(ARCapture *capture) { return capture->microphone; }

static OSStatus property_changed(AudioObjectID object, UInt32 count, const AudioObjectPropertyAddress *addresses, void *user_data) {
    ARCapture *capture = user_data;
    atomic_store_explicit(&capture->changed, 1, memory_order_relaxed);
    return noErr;
}

static OSStatus watch_property(ARCapture *capture, AudioObjectID object, AudioObjectPropertySelector selector, AudioObjectPropertyScope scope) {
    if (capture->watch_count >= AR_MAX_WATCHES) return kAudioHardwareUnspecifiedError;
    ARWatch watch = {object, {selector, scope, kAudioObjectPropertyElementMain}};
    OSStatus status = AudioObjectAddPropertyListener(object, &watch.address, property_changed, capture);
    if (status == noErr) capture->watches[capture->watch_count++] = watch;
    return status;
}

static int authorize_microphone(char *error, size_t error_size) {
    AVAuthorizationStatus status = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
    if (status == AVAuthorizationStatusNotDetermined) {
        dispatch_semaphore_t signal = dispatch_semaphore_create(0);
        dispatch_retain(signal);
        [AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL granted) {
            dispatch_semaphore_signal(signal);
            dispatch_release(signal);
        }];
        long pending = dispatch_semaphore_wait(signal, dispatch_time(DISPATCH_TIME_NOW, 60 * NSEC_PER_SEC));
        dispatch_release(signal);
        if (pending) {
            snprintf(error, error_size, "microphone permission request is pending");
            return 0;
        }
        status = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
    }
    if (status != AVAuthorizationStatusAuthorized) {
        snprintf(error, error_size, "microphone permission denied or restricted; enable microphone access in System Settings > Privacy & Security");
        return 0;
    }
    return 1;
}

static int valid_format(const AudioStreamBasicDescription *format) {
    UInt32 bytes = sizeof(float) * ((format->mFormatFlags & kAudioFormatFlagIsNonInterleaved) ? 1 : format->mChannelsPerFrame);
    return format->mFormatID == kAudioFormatLinearPCM && format->mBitsPerChannel == 32 &&
        (format->mFormatFlags & (kAudioFormatFlagIsFloat | kAudioFormatFlagIsPacked | kAudioFormatFlagIsBigEndian)) ==
            (kAudioFormatFlagIsFloat | kAudioFormatFlagIsPacked) && format->mBytesPerFrame == bytes && format->mFramesPerPacket == 1;
}

static OSStatus configure_streams(ARCapture *capture, uint32_t minimum_channels, char *error, size_t error_size) {
    AudioStreamID streams[AR_MAX_STREAMS];
    UInt32 size = sizeof(streams);
    OSStatus status = get_property(capture->aggregate, kAudioDevicePropertyStreams, kAudioObjectPropertyScopeInput, &size, streams);
    if (status != noErr) return status;
    uint32_t channels = 0;
    for (unsigned i = 0; i < size / sizeof(AudioStreamID); i++) {
        AudioStreamBasicDescription format;
        UInt32 format_size = sizeof(format);
        status = get_property(streams[i], kAudioStreamPropertyVirtualFormat, kAudioObjectPropertyScopeGlobal, &format_size, &format);
        if (status != noErr) return status;
        // HAL aggregate callbacks share the aggregate frame clock. A drift-
        // compensated tap can still advertise its original stream rate (48 kHz
        // on a 16 kHz Bluetooth aggregate). Buffer lengths are checked in IO;
        // AudioDevice's nominal rate determines the mixed PCM time base.
        if (!valid_format(&format)) {
            AudioObjectPropertyAddress address = {kAudioStreamPropertyVirtualFormat, kAudioObjectPropertyScopeGlobal, kAudioObjectPropertyElementMain};
            Boolean settable = false;
            status = AudioObjectIsPropertySettable(streams[i], &address, &settable);
            if (status != noErr || !settable) {
                snprintf(error, error_size, "aggregate stream %u format is not settable: id=%u flags=%u bits=%u bytes=%u channels=%u", streams[i], format.mFormatID, format.mFormatFlags, format.mBitsPerChannel, format.mBytesPerFrame, format.mChannelsPerFrame);
                return kAudioHardwareUnsupportedOperationError;
            }
            UInt32 noninterleaved = format.mFormatFlags & kAudioFormatFlagIsNonInterleaved;
            format.mFormatID = kAudioFormatLinearPCM;
            format.mSampleRate = capture->rate;
            format.mFormatFlags = kAudioFormatFlagsNativeFloatPacked | noninterleaved;
            format.mBitsPerChannel = 32;
            format.mFramesPerPacket = 1;
            format.mBytesPerFrame = sizeof(float) * (noninterleaved ? 1 : format.mChannelsPerFrame);
            format.mBytesPerPacket = format.mBytesPerFrame;
            status = AudioObjectSetPropertyData(streams[i], &address, 0, NULL, sizeof(format), &format);
            if (status != noErr) {
                snprintf(error, error_size, "set aggregate stream %u to %.0f Hz float PCM: Core Audio status %d", streams[i], capture->rate, (int)status);
                return status;
            }
            format_size = sizeof(format);
            status = get_property(streams[i], kAudioStreamPropertyVirtualFormat, kAudioObjectPropertyScopeGlobal, &format_size, &format);
            if (status != noErr || !valid_format(&format)) return kAudioHardwareUnsupportedOperationError;
        }
        channels += format.mChannelsPerFrame;
        status = watch_property(capture, streams[i], kAudioStreamPropertyVirtualFormat, kAudioObjectPropertyScopeGlobal);
        if (status != noErr) return status;
    }
    if (channels != minimum_channels) return kAudioHardwareBadDeviceError;
    capture->channels = channels;
    return noErr;
}

static OSStatus input_channels(AudioObjectID device, uint32_t *channels) {
    AudioObjectPropertyAddress address = {kAudioDevicePropertyStreamConfiguration, kAudioObjectPropertyScopeInput, kAudioObjectPropertyElementMain};
    UInt32 size = 0;
    OSStatus status = AudioObjectGetPropertyDataSize(device, &address, 0, NULL, &size);
    if (status != noErr) return status;
    AudioBufferList *buffers = calloc(1, size);
    if (!buffers) return kAudioHardwareUnspecifiedError;
    status = AudioObjectGetPropertyData(device, &address, 0, NULL, &size, buffers);
    *channels = 0;
    if (status == noErr) for (UInt32 i = 0; i < buffers->mNumberBuffers; i++) *channels += buffers->mBuffers[i].mNumberChannels;
    free(buffers);
    return status;
}

static OSStatus create_aggregate(ARCapture *capture) {
    UInt32 size = sizeof(capture->microphone);
    OSStatus status = get_property(kAudioObjectSystemObject, kAudioHardwarePropertyDefaultInputDevice, kAudioObjectPropertyScopeGlobal, &size, &capture->microphone);
    if (status != noErr || capture->microphone == kAudioObjectUnknown) return kAudioHardwareBadDeviceError;
    CFStringRef microphone_uid = NULL;
    size = sizeof(microphone_uid);
    status = get_property(capture->microphone, kAudioDevicePropertyDeviceUID, kAudioObjectPropertyScopeGlobal, &size, &microphone_uid);
    if (status != noErr) return status;
    size = sizeof(capture->rate);
    status = get_property(capture->microphone, kAudioDevicePropertyNominalSampleRate, kAudioObjectPropertyScopeGlobal, &size, &capture->rate);
    if (status != noErr) { CFRelease(microphone_uid); return status; }
    CATapDescription *description = [[CATapDescription alloc] initMonoGlobalTapButExcludeProcesses:@[]];
    description.name = @"Ambient Recorder Playback";
    description.privateTap = YES;
    description.muteBehavior = CATapUnmuted;
    status = AudioHardwareCreateProcessTap(description, &capture->tap);
    if (status == noErr) {
        NSDictionary *aggregate = @{
            @kAudioAggregateDeviceUIDKey: [[NSUUID UUID] UUIDString],
            @kAudioAggregateDeviceNameKey: @"Ambient Recorder Capture",
            @kAudioAggregateDeviceIsPrivateKey: @YES,
            @kAudioAggregateDeviceIsStackedKey: @NO,
            @kAudioAggregateDeviceTapAutoStartKey: @NO,
            @kAudioAggregateDeviceMainSubDeviceKey: (NSString *)microphone_uid,
            @kAudioAggregateDeviceSubDeviceListKey: @[@{@kAudioSubDeviceUIDKey: (NSString *)microphone_uid}],
            @kAudioAggregateDeviceTapListKey: @[@{@kAudioSubTapUIDKey: [description.UUID UUIDString], @kAudioSubTapDriftCompensationKey: @YES}]
        };
        status = AudioHardwareCreateAggregateDevice((CFDictionaryRef)aggregate, &capture->aggregate);
    }
    [description release];
    CFRelease(microphone_uid);
    if (status == noErr) {
        // Encode the aggregate's frame clock rather than a substream's format.
        size = sizeof(capture->rate);
        status = get_property(capture->aggregate, kAudioDevicePropertyNominalSampleRate, kAudioObjectPropertyScopeGlobal, &size, &capture->rate);
    }
    return status;
}

ARCapture *ar_capture_open(char *error, size_t error_size, int32_t *cleanup_status) {
    *cleanup_status = noErr;
    error[0] = 0;
    @autoreleasepool {
        if (@available(macOS 14.2, *)) {} else { snprintf(error, error_size, "macOS 14.2 or later is required"); return NULL; }
        if (!authorize_microphone(error, error_size)) return NULL;
        ARCapture *capture = calloc(1, sizeof(*capture));
        if (!capture) { snprintf(error, error_size, "allocate capture buffer"); return NULL; }
        const char *operation = "create microphone and playback aggregate";
        OSStatus status = create_aggregate(capture);
        if (status == noErr) {
            operation = "negotiate aggregate PCM format";
            uint32_t channels = 0;
            status = input_channels(capture->microphone, &channels);
            AudioStreamBasicDescription tap_format;
            UInt32 size = sizeof(tap_format);
            if (status == noErr) status = get_property(capture->tap, kAudioTapPropertyFormat, kAudioObjectPropertyScopeGlobal, &size, &tap_format);
            if (status == noErr) status = configure_streams(capture, channels + tap_format.mChannelsPerFrame, error, error_size);
        }
        if (status == noErr) status = watch_property(capture, kAudioObjectSystemObject, kAudioHardwarePropertyDefaultInputDevice, kAudioObjectPropertyScopeGlobal);
        if (status == noErr) status = watch_property(capture, capture->microphone, kAudioDevicePropertyDeviceIsAlive, kAudioObjectPropertyScopeGlobal);
        if (status == noErr) status = watch_property(capture, capture->microphone, kAudioDevicePropertyNominalSampleRate, kAudioObjectPropertyScopeGlobal);
        if (status == noErr) status = watch_property(capture, capture->microphone, kAudioDevicePropertyStreamConfiguration, kAudioObjectPropertyScopeInput);
        if (status == noErr) {
            operation = "register capture callback";
            status = AudioDeviceCreateIOProcID(capture->aggregate, capture_io, capture, &capture->io);
        }
        if (status == noErr) {
            operation = "start microphone and playback capture (check both audio permissions)";
            atomic_store_explicit(&capture->changed, 0, memory_order_relaxed);
            status = AudioDeviceStart(capture->aggregate, capture->io);
            if (status == noErr) capture->running = 1;
        }
        if (status == noErr) return capture;
        int32_t cleanup = ar_capture_close(capture);
        *cleanup_status = cleanup;
        if (!error[0]) snprintf(error, error_size, "%s: Core Audio status %d; cleanup status %d", operation, (int)status, (int)cleanup);
        return NULL;
    }
}

int32_t ar_capture_stop(ARCapture *capture) {
    if (!capture->running) return noErr;
    OSStatus status = AudioDeviceStop(capture->aggregate, capture->io);
    if (status == noErr) capture->running = 0;
    return status;
}

int32_t ar_capture_close(ARCapture *capture) {
    OSStatus failure = ar_capture_stop(capture);
    if (capture->io) {
        OSStatus status = AudioDeviceDestroyIOProcID(capture->aggregate, capture->io);
        if (status != noErr) failure = status;
    }
    for (unsigned i = 0; i < capture->watch_count; i++) {
        ARWatch *watch = &capture->watches[i];
        OSStatus status = AudioObjectRemovePropertyListener(watch->object, &watch->address, property_changed, capture);
        if (status != noErr) failure = status;
    }
    if (capture->aggregate) {
        OSStatus status = AudioHardwareDestroyAggregateDevice(capture->aggregate);
        if (status != noErr) failure = status;
    }
    if (capture->tap) {
        OSStatus status = AudioHardwareDestroyProcessTap(capture->tap);
        if (status != noErr) failure = status;
    }
    // A failed deregistration can leave HAL callbacks referencing this allocation.
    // Retain it until process exit rather than free memory still owned by the HAL.
    if (failure == noErr) free(capture);
    return failure;
}
