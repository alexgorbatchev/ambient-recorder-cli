#include "../native.m"
#include "../devices.m"
#include <assert.h>

int main(void) {
    ARDeviceMonitor monitor = {0};
    assert(!ar_monitor_changed(&monitor));
    devices_changed(kAudioObjectSystemObject, 0, NULL, &monitor);
    assert(ar_monitor_changed(&monitor));
    assert(!ar_monitor_changed(&monitor));
    char *name = copy_cf_string(CFSTR("USB Microphone – Desk"));
    assert(name && strcmp(name, "USB Microphone – Desk") == 0);
    free(name);
    assert(strcmp(transport_name(kAudioDeviceTransportTypeBluetooth), "Bluetooth") == 0);
    assert(strcmp(transport_name(kAudioDeviceTransportTypeUSB), "USB") == 0);
    assert(strcmp(transport_name(kAudioDeviceTransportTypeBuiltIn), "Built-in") == 0);
    assert(strcmp(transport_name(kAudioDeviceTransportTypeDisplayPort), "DisplayPort") == 0);
#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 270000
    assert(strcmp(transport_name(kAudioDeviceTransportTypeRemoteScreen), "Screen sharing") == 0);
    assert(strcmp(transport_name(kAudioDeviceTransportTypeRemoteStreaming), "Remote streaming") == 0);
#endif
    assert(strcmp(transport_name(UINT32_MAX), "Unavailable") == 0);
    assert(strcmp(terminal_name(kAudioStreamTerminalTypeHeadsetMicrophone), "Headset microphone") == 0);
    assert(strcmp(terminal_name(kAudioStreamTerminalTypeMicrophone), "Microphone") == 0);
    assert(strcmp(terminal_name(UINT32_MAX), "Unavailable") == 0);
    ARCapture *capture = calloc(1, sizeof(*capture));
    assert(capture);
    capture->channels = 3;
    assert(allocate_blocks(capture) == noErr);
    assert(ar_capture_channels(capture) == 3);
    float microphone[] = {0.6f, 0.3f, -0.3f, -0.6f};
    float playback[] = {0.0f, -0.2f};
    struct { UInt32 count; AudioBuffer buffers[2]; } input = {
        2, {{2, sizeof(microphone), microphone}, {1, sizeof(playback), playback}}
    };
    AudioTimeStamp timestamp = {.mHostTime = 1234, .mSampleTime = 4567, .mFlags = kAudioTimeStampHostTimeValid | kAudioTimeStampSampleTimeValid};
    capture_io(0, &timestamp, (AudioBufferList *)&input, &timestamp, NULL, NULL, capture);
    float pcm[AR_MAX_FRAMES * 3] = {0};
    ARChunk chunk;
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES, &chunk) == 1);
    assert(chunk.frames == 2 && chunk.host_time == 1234 && chunk.sample_time == 4567);
    assert(fabsf(pcm[0] - 0.6f) < 0.00001f && fabsf(pcm[1] - 0.3f) < 0.00001f && pcm[2] == 0.0f);
    assert(fabsf(pcm[3] + 0.3f) < 0.00001f && fabsf(pcm[4] + 0.6f) < 0.00001f && fabsf(pcm[5] + 0.2f) < 0.00001f);
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES, &chunk) == 0);

    // Interleaved and separate HAL buffers preserve the same source order.
    float first[] = {0.6f, -0.3f}, second[] = {0.3f, -0.6f};
    struct { UInt32 count; AudioBuffer buffers[3]; } planar = {
        3, {{1, sizeof(first), first}, {1, sizeof(second), second}, {1, sizeof(playback), playback}}
    };
    capture_io(0, &timestamp, (AudioBufferList *)&planar, &timestamp, NULL, NULL, capture);
    assert(ar_capture_read(capture, pcm, 5, &chunk) == -1);
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES * 3, &chunk) == 1);
    assert(pcm[0] == first[0] && pcm[1] == second[0] && pcm[2] == playback[0]);
    assert(pcm[3] == first[1] && pcm[4] == second[1] && pcm[5] == playback[1]);
    input.buffers[0].mData = NULL;
    capture_io(0, &timestamp, (AudioBufferList *)&input, &timestamp, NULL, NULL, capture);
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES * 3, &chunk) == 1);
    assert(pcm[0] == 0.f && pcm[1] == 0.f && pcm[3] == 0.f && pcm[4] == 0.f && pcm[5] == playback[1]);
    input.buffers[0].mData = microphone;
    microphone[0] = NAN; microphone[1] = 2.f; microphone[2] = -2.f; microphone[3] = INFINITY;
    capture_io(0, &timestamp, (AudioBufferList *)&input, &timestamp, NULL, NULL, capture);
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES * 3, &chunk) == 1);
    assert(pcm[0] == 0.f && pcm[1] == 1.f && pcm[3] == -1.f && pcm[4] == 0.f && pcm[5] == playback[1]);

    input.buffers[0].mDataByteSize = 0;
    atomic_store(&capture->changed, 0);
    capture_io(0, &timestamp, (AudioBufferList *)&input, &timestamp, NULL, NULL, capture);
    assert(ar_capture_changed(capture));
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES, &chunk) == 0);
    input.buffers[0].mDataByteSize = sizeof(microphone);
    atomic_store(&capture->changed, 0);

    // Invalid channel layouts must stop capture before accessing PCM data.
    input.buffers[0].mNumberChannels = 4;
    capture_io(0, &timestamp, (AudioBufferList *)&input, &timestamp, NULL, NULL, capture);
    assert(ar_capture_changed(capture));
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES * 3, &chunk) == 0);
    input.buffers[0].mNumberChannels = 2;
    input.buffers[1].mNumberChannels = 2;
    atomic_store(&capture->changed, 0);
    capture_io(0, &timestamp, (AudioBufferList *)&input, &timestamp, NULL, NULL, capture);
    assert(ar_capture_changed(capture));
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES * 3, &chunk) == 0);
    input.buffers[1].mNumberChannels = 1;
    atomic_store(&capture->changed, 0);

    for (int i = 0; i < AR_QUEUE_BLOCKS + 2; i++) capture_io(0, &timestamp, (AudioBufferList *)&input, &timestamp, NULL, NULL, capture);
    assert(ar_capture_dropped(capture) == 4);
    for (int i = 0; i < AR_QUEUE_BLOCKS; i++) assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES, &chunk) == 1);
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES, &chunk) == 0);

    input.buffers[1].mDataByteSize = sizeof(float);
    capture_io(0, &timestamp, (AudioBufferList *)&input, &timestamp, NULL, NULL, capture);
    assert(ar_capture_changed(capture));
    assert(ar_capture_read(capture, pcm, AR_MAX_FRAMES, &chunk) == 0);
    assert(ar_capture_close(capture) == noErr);
    ARCapture invalid = {0};
    assert(allocate_blocks(&invalid) == EINVAL);
    invalid.channels = UINT32_MAX;
    assert(allocate_blocks(&invalid) == EINVAL);
    return 0;
}
