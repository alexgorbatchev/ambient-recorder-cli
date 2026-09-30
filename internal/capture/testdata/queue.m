#include "../native.m"
#include <assert.h>

int main(void) {
    char *name = copy_cf_string(CFSTR("USB Microphone – Desk"));
    assert(name && strcmp(name, "USB Microphone – Desk") == 0);
    free(name);
    ARCapture *capture = calloc(1, sizeof(*capture));
    assert(capture);
    capture->channels = 3;
    float microphone[] = {0.6f, -0.3f};
    float playback[] = {0.3f, 0.0f, -0.6f, 0.0f};
    struct { UInt32 count; AudioBuffer buffers[2]; } input = {
        2, {{1, sizeof(microphone), microphone}, {2, sizeof(playback), playback}}
    };
    AudioTimeStamp timestamp = {.mHostTime = 1234, .mSampleTime = 4567, .mFlags = kAudioTimeStampHostTimeValid | kAudioTimeStampSampleTimeValid};
    capture_io(0, &timestamp, (AudioBufferList *)&input, &timestamp, NULL, NULL, capture);
    float pcm[AR_MAX_FRAMES];
    ARChunk chunk;
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES, &chunk) == 1);
    assert(chunk.frames == 2 && chunk.host_time == 1234 && chunk.sample_time == 4567);
    assert(fabsf(pcm[0] - 0.3f) < 0.00001f && fabsf(pcm[1] + 0.3f) < 0.00001f);
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES, &chunk) == 0);

    input.buffers[0].mDataByteSize = 0;
    atomic_store(&capture->changed, 0);
    capture_io(0, &timestamp, (AudioBufferList *)&input, &timestamp, NULL, NULL, capture);
    assert(ar_capture_changed(capture));
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES, &chunk) == 0);
    input.buffers[0].mDataByteSize = sizeof(microphone);
    atomic_store(&capture->changed, 0);

    for (int i = 0; i < AR_QUEUE_BLOCKS + 2; i++) capture_io(0, &timestamp, (AudioBufferList *)&input, &timestamp, NULL, NULL, capture);
    assert(ar_capture_dropped(capture) == 4);
    for (int i = 0; i < AR_QUEUE_BLOCKS; i++) assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES, &chunk) == 1);
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES, &chunk) == 0);

    input.buffers[1].mDataByteSize = sizeof(float);
    capture_io(0, &timestamp, (AudioBufferList *)&input, &timestamp, NULL, NULL, capture);
    assert(ar_capture_changed(capture));
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES, &chunk) == 0);
    free(capture);
    return 0;
}
