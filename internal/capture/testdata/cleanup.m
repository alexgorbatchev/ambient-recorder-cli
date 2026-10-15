#include "../native.m"
#include <assert.h>
#include <unistd.h>

// Real HAL objects and listeners, without opening a microphone or starting IO.
int main(void) {
    @autoreleasepool {
        for (int removed = 0; removed < 2; removed++) {
            ARCapture *capture = calloc(1, sizeof(*capture));
            assert(capture);
            NSString *uid = [[NSUUID UUID] UUIDString];
            NSDictionary *description = @{
                @kAudioAggregateDeviceNameKey: @"Ambient Recorder Cleanup Test",
                @kAudioAggregateDeviceUIDKey: uid,
                @kAudioAggregateDeviceIsPrivateKey: @YES,
            };
            AudioObjectID device = kAudioObjectUnknown;
            assert(AudioHardwareCreateAggregateDevice((CFDictionaryRef)description, &device) == noErr);
            assert(watch_property(capture, device, kAudioDevicePropertyNominalSampleRate, kAudioObjectPropertyScopeGlobal) == noErr);
            if (removed) {
                assert(AudioHardwareDestroyAggregateDevice(device) == noErr);
                OSStatus status = noErr;
                for (unsigned attempt = 0; attempt < 100 && status == noErr; attempt++) {
                    double rate;
                    UInt32 size = sizeof(rate);
                    status = get_property(device, kAudioDevicePropertyNominalSampleRate, kAudioObjectPropertyScopeGlobal, &size, &rate);
                    usleep(10000);
                }
                assert(status == kAudioHardwareBadObjectError);
            } else {
                capture->aggregate = device;
            }
            int32_t status = ar_capture_close(capture);
            fprintf(stderr, "cleanup with watched object %s: %d\n", removed ? "removed" : "present", status);
            assert(status == noErr);
        }
    }
    return 0;
}
