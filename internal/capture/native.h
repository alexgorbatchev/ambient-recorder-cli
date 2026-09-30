#ifndef AMBIENT_CAPTURE_H
#define AMBIENT_CAPTURE_H
#include <stdint.h>
#include <stddef.h>

enum { AR_MAX_FRAMES = 8192, AR_QUEUE_BLOCKS = 256 };
typedef struct ARCapture ARCapture;
typedef struct {
    uint32_t frames;
    uint64_t host_time;
    double sample_time;
    uint32_t time_flags;
} ARChunk;

ARCapture *ar_capture_open(char *error, size_t error_size, int32_t *cleanup_status);
int32_t ar_capture_close(ARCapture *capture);
int32_t ar_capture_stop(ARCapture *capture);
int ar_capture_read(ARCapture *capture, float *pcm, uint32_t capacity, ARChunk *chunk);
uint64_t ar_capture_dropped(ARCapture *capture);
int ar_capture_changed(ARCapture *capture);
double ar_capture_rate(ARCapture *capture);
uint32_t ar_capture_microphone(ARCapture *capture);
// The caller frees the returned UTF-8 string; NULL means metadata unavailable.
char *ar_capture_microphone_name(ARCapture *capture);
#endif
