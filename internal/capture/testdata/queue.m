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
    assert(strcmp(transport_name(UINT32_MAX), "Unavailable") == 0);
    assert(strcmp(terminal_name(kAudioStreamTerminalTypeHeadsetMicrophone), "Headset microphone") == 0);
    assert(strcmp(terminal_name(kAudioStreamTerminalTypeMicrophone), "Microphone") == 0);
    assert(strcmp(terminal_name(UINT32_MAX), "Unavailable") == 0);
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
