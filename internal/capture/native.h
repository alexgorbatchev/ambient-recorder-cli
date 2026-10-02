#ifndef AMBIENT_CAPTURE_H
#define AMBIENT_CAPTURE_H
#include <stdint.h>
#include <stddef.h>

enum { AR_MAX_FRAMES = 8192, AR_QUEUE_BLOCKS = 256, AR_MAX_STREAMS = 32 };
typedef struct ARCapture ARCapture;
typedef struct {
    uint32_t frames;
    uint64_t host_time;
    double sample_time;
    uint32_t time_flags;
} ARChunk;

ARCapture *ar_capture_open(uint32_t microphone, char *error, size_t error_size, int32_t *cleanup_status);
int32_t ar_capture_close(ARCapture *capture);
int32_t ar_capture_stop(ARCapture *capture);
int ar_capture_read(ARCapture *capture, float *pcm, uint32_t capacity, ARChunk *chunk);
uint64_t ar_capture_dropped(ARCapture *capture);
int ar_capture_changed(ARCapture *capture);
double ar_capture_rate(ARCapture *capture);
uint32_t ar_capture_microphone(ARCapture *capture);
uint32_t ar_capture_aggregate(ARCapture *capture);
// The caller frees the returned UTF-8 string; NULL means metadata unavailable.
char *ar_device_string(uint32_t device, uint32_t property);
const char *ar_device_transport(uint32_t device);
const char *ar_device_type(uint32_t device);
int32_t ar_microphones(uint32_t **devices, uint32_t *count, uint32_t *default_input);
typedef struct ARDeviceMonitor ARDeviceMonitor;
ARDeviceMonitor *ar_monitor_open(int32_t *status, int32_t *cleanup_status);
int ar_monitor_changed(ARDeviceMonitor *monitor);
int32_t ar_monitor_close(ARDeviceMonitor *monitor);
#endif
