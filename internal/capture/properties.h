#ifndef AMBIENT_PROPERTIES_H
#define AMBIENT_PROPERTIES_H
#include <CoreAudio/CoreAudio.h>
#include <stdlib.h>
#include <limits.h>

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

#endif
