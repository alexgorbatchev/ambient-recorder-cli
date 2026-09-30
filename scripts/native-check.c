#include <opusenc.h>

int main(void) {
    OggOpusComments *comments = ope_comments_create();
    if (!comments) return 1;
    int error = 0;
    OggOpusEnc *encoder = ope_encoder_create_pull(comments, 48000, 1, 0, &error);
    ope_comments_destroy(comments);
    if (!encoder || error != OPE_OK) return 2;
    error = ope_encoder_drain(encoder);
    ope_encoder_destroy(encoder);
    return error == OPE_OK ? 0 : 3;
}
