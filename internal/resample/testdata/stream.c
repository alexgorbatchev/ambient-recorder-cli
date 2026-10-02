#include "native.h"
#include <assert.h>
#include <math.h>
#include <stdint.h>
#include <stdio.h>

static uint64_t drain(ARResampler *r, int finish) {
    float output[1024];
    uint64_t total = 0;
    for (;;) {
        uint32_t frames = sizeof(output) / sizeof(output[0]);
        int32_t status = ar_resampler_read(r, output, &frames, finish);
        assert(status == 0 || (!finish && status == AR_RESAMPLE_NEED_INPUT));
        assert(frames <= sizeof(output) / sizeof(output[0]));
        for (uint32_t i = 0; i < frames; i++) assert(isfinite(output[i]));
        total += frames;
        if (status == AR_RESAMPLE_NEED_INPUT || frames == 0) return total;
    }
}

int main(void) {
    // Production bypasses the native converter at 48 kHz (covered by Go tests).
    const uint32_t rates[] = {16000, 44100, 96000};
    const uint32_t blocks[] = {137, AR_RESAMPLE_CAPACITY};
    for (unsigned i = 0; i < sizeof(rates)/sizeof(rates[0]); i++) {
        for (unsigned j = 0; j < sizeof(blocks)/sizeof(blocks[0]); j++) {
            int32_t status;
            ARResampler *r = ar_resampler_open(rates[i], &status);
            if (!r || status) fprintf(stderr, "rate=%u block=%u status=%d\n", rates[i], blocks[j], status);
            assert(r && status == 0);
            uint64_t total = 0;
            float input[AR_RESAMPLE_CAPACITY];
            for (uint32_t offset = 0; offset < rates[i];) {
                uint32_t n = rates[i] - offset;
                if (n > blocks[j]) n = blocks[j];
                for (uint32_t k = 0; k < n; k++) input[k] = 0.5*sin(2*M_PI*880*(offset+k)/rates[i]);
                assert(ar_resampler_push(r, input, n) == 0);
                total += drain(r, 0);
                offset += n;
            }
            total += drain(r, 1);
            assert(total == AR_RESAMPLE_RATE);
            assert(ar_resampler_close(r) == 0);
        }
    }
    return 0;
}
