#ifndef SPEECH_TTS_WINDOWS_H
#define SPEECH_TTS_WINDOWS_H

#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
    char* id;
    char* name;
    char* language;
    char* gender;
} VoiceInfo;

void* tts_create();
void tts_destroy(void* handle);
VoiceInfo* tts_list_voices(void* handle, int* count);
void tts_free_voices(VoiceInfo* voices, int count);
int tts_synthesize(void* handle, const char* text, const char* voice_id,
                   float speed, float pitch, float volume,
                   char** audio_data, int* audio_size, int* sample_rate);
void tts_free_audio(char* audio_data);

#ifdef __cplusplus
}
#endif

#endif // SPEECH_TTS_WINDOWS_H
