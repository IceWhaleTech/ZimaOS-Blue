#include "hifigan.h"
#include <cstring>
#include <cstdlib>

// Stub implementation - passthrough vocoder
// In production, this would load and use the actual HiFi-GAN model

struct HiFiGANModel {
    int sample_rate;
};

extern "C" {

hifigan_t* hifigan_init(const char* model_path, int sample_rate) {
    if (!model_path || sample_rate <= 0) {
        return nullptr;
    }

    hifigan_t* vocoder = (hifigan_t*)malloc(sizeof(hifigan_t));
    if (!vocoder) {
        return nullptr;
    }

    HiFiGANModel* model = (HiFiGANModel*)malloc(sizeof(HiFiGANModel));
    if (!model) {
        free(vocoder);
        return nullptr;
    }

    model->sample_rate = sample_rate;
    vocoder->model = model;
    vocoder->sample_rate = sample_rate;

    return vocoder;
}

int hifigan_process(hifigan_t* vocoder, const int16_t* input, int input_len,
                    int16_t* output, int output_capacity) {
    if (!vocoder || !input || !output || input_len <= 0) {
        return -1;
    }

    // Passthrough: copy input to output
    if (input_len > output_capacity) {
        return -1;
    }

    memcpy(output, input, input_len * sizeof(int16_t));
    return input_len;
}

void hifigan_cleanup(hifigan_t* vocoder) {
    if (!vocoder) {
        return;
    }

    if (vocoder->model) {
        free(vocoder->model);
        vocoder->model = nullptr;
    }

    free(vocoder);
}

} // extern "C"
