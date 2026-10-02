#include "native.h"
#include "properties.h"
#include <stdatomic.h>

int32_t ar_microphones(uint32_t **devices, uint32_t *count, uint32_t *default_input) {
    *devices = NULL;
    *count = 0;
    *default_input = kAudioObjectUnknown;
    UInt32 size = sizeof(*default_input);
    // No default is valid during a device transition; other inputs still work.
    if (get_property(kAudioObjectSystemObject, kAudioHardwarePropertyDefaultInputDevice, kAudioObjectPropertyScopeGlobal, &size, default_input) != noErr)
        *default_input = kAudioObjectUnknown;
    AudioObjectPropertyAddress address = {kAudioHardwarePropertyDevices, kAudioObjectPropertyScopeGlobal, kAudioObjectPropertyElementMain};
    size = 0;
    OSStatus status = AudioObjectGetPropertyDataSize(kAudioObjectSystemObject, &address, 0, NULL, &size);
    if (status != noErr || !size) return status;
    AudioObjectID *all = malloc(size);
    if (!all) return kAudioHardwareUnspecifiedError;
    status = AudioObjectGetPropertyData(kAudioObjectSystemObject, &address, 0, NULL, &size, all);
    if (status != noErr) { free(all); return status; }
    for (unsigned i = 0; i < size / sizeof(AudioObjectID); i++) {
        UInt32 alive = 0, alive_size = sizeof(alive), channels = 0;
        if (get_property(all[i], kAudioDevicePropertyDeviceIsAlive, kAudioObjectPropertyScopeGlobal, &alive_size, &alive) != noErr || !alive)
            continue;
        if (input_channels(all[i], &channels) == noErr && channels) all[(*count)++] = all[i];
    }
    *devices = all;
    return noErr;
}

struct ARDeviceMonitor {
    _Atomic int changed;
    unsigned installed;
};

static const AudioObjectPropertySelector monitor_properties[] = {
    kAudioHardwarePropertyDevices, kAudioHardwarePropertyDefaultInputDevice
};

static OSStatus devices_changed(AudioObjectID object, UInt32 count, const AudioObjectPropertyAddress *addresses, void *data) {
    ARDeviceMonitor *monitor = data;
    atomic_store_explicit(&monitor->changed, 1, memory_order_relaxed);
    return noErr;
}

ARDeviceMonitor *ar_monitor_open(int32_t *status, int32_t *cleanup_status) {
    *cleanup_status = noErr;
    ARDeviceMonitor *monitor = calloc(1, sizeof(*monitor));
    if (!monitor) { *status = kAudioHardwareUnspecifiedError; return NULL; }
    atomic_store(&monitor->changed, 1);
    for (unsigned i = 0; i < sizeof(monitor_properties) / sizeof(*monitor_properties); i++) {
        AudioObjectPropertyAddress address = {monitor_properties[i], kAudioObjectPropertyScopeGlobal, kAudioObjectPropertyElementMain};
        *status = AudioObjectAddPropertyListener(kAudioObjectSystemObject, &address, devices_changed, monitor);
        if (*status != noErr) {
            *cleanup_status = ar_monitor_close(monitor);
            return NULL;
        }
        monitor->installed++;
    }
    *status = noErr;
    return monitor;
}

int ar_monitor_changed(ARDeviceMonitor *monitor) {
    return atomic_exchange_explicit(&monitor->changed, 0, memory_order_relaxed);
}

int32_t ar_monitor_close(ARDeviceMonitor *monitor) {
    OSStatus failure = noErr;
    for (unsigned i = 0; i < monitor->installed; i++) {
        AudioObjectPropertyAddress address = {monitor_properties[i], kAudioObjectPropertyScopeGlobal, kAudioObjectPropertyElementMain};
        OSStatus status = AudioObjectRemovePropertyListener(kAudioObjectSystemObject, &address, devices_changed, monitor);
        if (status != noErr) failure = status;
    }
    // Failed removal may leave callbacks alive; process restart releases memory.
    if (failure == noErr) free(monitor);
    return failure;
}

char *ar_device_string(uint32_t device, uint32_t property) {
    CFStringRef name = NULL;
    UInt32 size = sizeof(name);
    OSStatus status = get_property(device, property, kAudioObjectPropertyScopeGlobal, &size, &name);
    if (status != noErr || !name) return NULL;
    char *value = copy_cf_string(name);
    CFRelease(name);
    return value;
}

static const char *transport_name(UInt32 type) {
    switch (type) {
        case kAudioDeviceTransportTypeBuiltIn: return "Built-in";
        case kAudioDeviceTransportTypeAggregate: return "Aggregate";
        case kAudioDeviceTransportTypeVirtual: return "Virtual";
        case kAudioDeviceTransportTypePCI: return "PCI";
        case kAudioDeviceTransportTypeUSB: return "USB";
        case kAudioDeviceTransportTypeFireWire: return "FireWire";
        case kAudioDeviceTransportTypeBluetooth: return "Bluetooth";
        case kAudioDeviceTransportTypeBluetoothLE: return "Bluetooth LE";
        case kAudioDeviceTransportTypeHDMI: return "HDMI";
        case kAudioDeviceTransportTypeDisplayPort: return "DisplayPort";
        case kAudioDeviceTransportTypeAirPlay: return "AirPlay";
        case kAudioDeviceTransportTypeAVB: return "AVB network";
        case kAudioDeviceTransportTypeThunderbolt: return "Thunderbolt";
        case kAudioDeviceTransportTypeContinuityCaptureWired: return "Continuity (wired)";
        case kAudioDeviceTransportTypeContinuityCaptureWireless: return "Continuity (wireless)";
        case kAudioDeviceTransportTypeRemoteScreen: return "Screen sharing";
        case kAudioDeviceTransportTypeRemoteStreaming: return "Remote streaming";
        default: return "Unavailable";
    }
}

const char *ar_device_transport(uint32_t device) {
    UInt32 type = kAudioDeviceTransportTypeUnknown;
    UInt32 size = sizeof(type);
    if (get_property(device, kAudioDevicePropertyTransportType, kAudioObjectPropertyScopeGlobal, &size, &type) != noErr)
        return "Unavailable";
    return transport_name(type);
}

static const char *terminal_name(UInt32 type) {
    switch (type) {
        case kAudioStreamTerminalTypeMicrophone: return "Microphone";
        case kAudioStreamTerminalTypeHeadsetMicrophone: return "Headset microphone";
        case kAudioStreamTerminalTypeReceiverMicrophone: return "Handset microphone";
        case kAudioStreamTerminalTypeLine: return "Line input";
        case kAudioStreamTerminalTypeDigitalAudioInterface: return "Digital audio interface";
        case kAudioStreamTerminalTypeHDMI: return "HDMI audio";
        case kAudioStreamTerminalTypeDisplayPort: return "DisplayPort audio";
        default: return "Unavailable";
    }
}

const char *ar_device_type(uint32_t device) {
    AudioStreamID streams[AR_MAX_STREAMS];
    UInt32 size = sizeof(streams);
    if (get_property(device, kAudioDevicePropertyStreams, kAudioObjectPropertyScopeInput, &size, streams) != noErr)
        return "Unavailable";
    UInt32 type = kAudioStreamTerminalTypeUnknown;
    for (unsigned i = 0; i < size / sizeof(AudioStreamID); i++) {
        UInt32 current = kAudioStreamTerminalTypeUnknown, value_size = sizeof(current);
        if (get_property(streams[i], kAudioStreamPropertyTerminalType, kAudioObjectPropertyScopeGlobal, &value_size, &current) != noErr)
            return "Unavailable";
        if (i && current != type) return "Multiple input types";
        type = current;
    }
    return terminal_name(type);
}
