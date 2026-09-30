#include <opus.h>
#include <stdint.h>
#include <stdio.h>

int main(void) {
    int error = 0;
    OpusDecoder *decoder = opus_decoder_create(48000, 1, &error);
    if (!decoder || error != OPUS_OK) return 1;
    unsigned char packet[65536];
    float pcm[5760];
    unsigned char size[4];
    while (fread(size, 1, 4, stdin) == 4) {
        uint32_t n = (uint32_t)size[0] | (uint32_t)size[1] << 8 | (uint32_t)size[2] << 16 | (uint32_t)size[3] << 24;
        if (n > sizeof(packet) || fread(packet, 1, n, stdin) != n) return 2;
        int frames = opus_decode_float(decoder, packet, (opus_int32)n, pcm, 5760, 0);
        if (frames < 0 || fwrite(pcm, sizeof(float), frames, stdout) != (size_t)frames) return 3;
    }
    opus_decoder_destroy(decoder);
    return ferror(stdin) || ferror(stdout) ? 4 : 0;
}
