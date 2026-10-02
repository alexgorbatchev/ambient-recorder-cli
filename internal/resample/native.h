#ifndef AMBIENT_RESAMPLE_H
#define AMBIENT_RESAMPLE_H
#include <stdint.h>
enum { AR_RESAMPLE_CAPACITY = 8192, AR_RESAMPLE_RATE = 48000, AR_RESAMPLE_NEED_INPUT = -1 };
typedef struct ARResampler ARResampler;
ARResampler *ar_resampler_open(double rate, int32_t *status);
int32_t ar_resampler_push(ARResampler *r, const float *pcm, uint32_t frames);
int32_t ar_resampler_read(ARResampler *r, float *pcm, uint32_t *frames, int finish);
int32_t ar_resampler_close(ARResampler *r);
#endif
