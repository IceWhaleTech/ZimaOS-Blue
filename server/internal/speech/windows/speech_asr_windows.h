#ifndef SPEECH_ASR_WINDOWS_H
#define SPEECH_ASR_WINDOWS_H

#ifdef __cplusplus
extern "C" {
#endif

void* asr_create(const char* language);
void asr_destroy(void* handle);
int asr_recognize(void* handle, const char* audio_data, int audio_size,
                  char** result_text, float* confidence);
void asr_free_result(char* text);

// Get installed speech recognition languages
// Returns a null-terminated array of language codes (e.g., "en-US", "zh-CN")
// Caller must free the returned array and each string using asr_free_languages
char** asr_get_installed_languages(int* count);
void asr_free_languages(char** languages, int count);

#ifdef __cplusplus
}
#endif

#endif // SPEECH_ASR_WINDOWS_H
