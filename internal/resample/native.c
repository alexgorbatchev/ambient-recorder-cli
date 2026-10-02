#include "native.h"
#include <AudioToolbox/AudioToolbox.h>
#include <errno.h>
#include <stdlib.h>
#include <string.h>

struct ARResampler {
    AudioConverterRef converter;
    float input[AR_RESAMPLE_CAPACITY];
    UInt32 frames;
    UInt32 offset;
    int writable;
    int finish;
};

static AudioStreamBasicDescription format(double rate) {
    return (AudioStreamBasicDescription){
        .mSampleRate = rate,
        .mFormatID = kAudioFormatLinearPCM,
        .mFormatFlags = kAudioFormatFlagsNativeFloatPacked,
        .mBytesPerPacket = sizeof(float),
        .mFramesPerPacket = 1,
        .mBytesPerFrame = sizeof(float),
        .mChannelsPerFrame = 1,
        .mBitsPerChannel = 8 * sizeof(float)
    };
}

static OSStatus input(AudioConverterRef converter, UInt32 *packets,
                      AudioBufferList *data, AudioStreamPacketDescription **descriptions,
                      void *user) {
    (void)converter;
    (void)descriptions;
    ARResampler *r = user;
    UInt32 n = r->frames - r->offset;
    if (n > *packets) n = *packets;
    *packets = n;
    data->mNumberBuffers = 1;
    data->mBuffers[0] = (AudioBuffer){1, n * sizeof(float), n ? r->input + r->offset : NULL};
    r->offset += n;
    if (n) return noErr;
    // Only this next callback releases the previous input buffer's lifetime.
    r->writable = 1;
    return r->finish ? noErr : AR_RESAMPLE_NEED_INPUT;
}

ARResampler *ar_resampler_open(double rate, int32_t *status) {
    ARResampler *r = calloc(1, sizeof(*r));
    if (!r) { *status = ENOMEM; return NULL; }
    AudioStreamBasicDescription source = format(rate), destination = format(AR_RESAMPLE_RATE);
    *status = AudioConverterNew(&source, &destination, &r->converter);
    if (*status) { free(r); return NULL; }
    UInt32 quality = kAudioConverterQuality_Medium;
    *status = AudioConverterSetProperty(r->converter, kAudioConverterSampleRateConverterQuality,
                                       sizeof(quality), &quality);
    if (*status) {
        OSStatus cleanup = AudioConverterDispose(r->converter);
        if (cleanup) *status = cleanup;
        free(r);
        return NULL;
    }
    // Normal priming preserves acquisition alignment without leading silence.
    // Its small look-ahead is drained at EOF, and temporarily missing input is
    // signalled with an error rather than masquerading as end of stream.
    r->writable = 1;
    return r;
}

int32_t ar_resampler_push(ARResampler *r, const float *pcm, uint32_t frames) {
    if (!r->writable || r->finish || frames > AR_RESAMPLE_CAPACITY) return EINVAL;
    memcpy(r->input, pcm, frames * sizeof(float));
    r->frames = frames;
    r->offset = 0;
    r->writable = 0;
    return noErr;
}

int32_t ar_resampler_read(ARResampler *r, float *pcm, uint32_t *frames, int finish) {
    r->finish = finish;
    AudioBufferList output = {.mNumberBuffers = 1,
        .mBuffers = {{1, *frames * sizeof(float), pcm}}};
    return AudioConverterFillComplexBuffer(r->converter, input, r, frames, &output, NULL);
}

int32_t ar_resampler_close(ARResampler *r) {
    OSStatus status = AudioConverterDispose(r->converter);
    free(r);
    return status;
}
