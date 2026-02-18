#ifndef HIFIGAN_H
#define HIFIGAN_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
    void* model;
    int sample_rate;
} hifigan_t;

// Initialize vocoder with model path
hifigan_t* hifigan_init(const char* model_path, int sample_rate);

// Process PCM audio (input_samples -> output_samples)
// Returns number of output samples, or -1 on error
int hifigan_process(hifigan_t* vocoder, const int16_t* input, int input_len, int16_t* output, int output_capacity);

// Cleanup
void hifigan_cleanup(hifigan_t* vocoder);

#ifdef __cplusplus
}
#endif

#endif // HIFIGAN_H
