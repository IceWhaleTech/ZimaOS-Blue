#include "speech_tts_windows.h"
#include <windows.h>
#include <sapi.h>
#include <sphelper.h>
#include <comdef.h>
#include <initguid.h>

// Define SAPI GUIDs
DEFINE_GUID(CLSID_SpVoice, 0x96749377, 0x3391, 0x11D2, 0x9E, 0xE3, 0x00, 0xC0, 0x4F, 0x79, 0x73, 0x96);
DEFINE_GUID(IID_ISpVoice, 0x6C44DF74, 0x72B9, 0x4992, 0xA1, 0xEC, 0xEF, 0x99, 0x6E, 0x04, 0x22, 0xD4);
DEFINE_GUID(CLSID_SpStream, 0x715D9C59, 0x4442, 0x11D2, 0x96, 0x05, 0x00, 0xC0, 0x4F, 0x8E, 0xE6, 0x28);
DEFINE_GUID(IID_ISpStream, 0x12E3CCA9, 0x7518, 0x44C5, 0xA5, 0xE7, 0xBA, 0x5A, 0x79, 0xCB, 0x92, 0x9E);
DEFINE_GUID(CLSID_SpObjectTokenCategory, 0xA910187F, 0x0C7A, 0x45AC, 0x92, 0xCC, 0x59, 0xED, 0xAF, 0xB7, 0x7B, 0x53);
DEFINE_GUID(IID_ISpObjectTokenCategory, 0x2D3D3845, 0x39AF, 0x4850, 0xBB, 0xF9, 0x40, 0xB4, 0x97, 0x80, 0x01, 0x1D);
DEFINE_GUID(CLSID_SpObjectToken, 0xEF411752, 0x3736, 0x4CB4, 0x9C, 0x8C, 0x8E, 0xF4, 0xCC, 0xB5, 0x8E, 0xFE);
DEFINE_GUID(SPDFID_WaveFormatEx, 0xC31ADBAE, 0x527F, 0x4FF5, 0xA2, 0x30, 0xF6, 0x2B, 0xB6, 0x1F, 0xF7, 0x0C);

// WAV file header structure
#pragma pack(push, 1)
struct WAVHeader {
    char riff[4];           // "RIFF"
    DWORD fileSize;         // File size - 8
    char wave[4];           // "WAVE"
    char fmt[4];            // "fmt "
    DWORD fmtSize;          // Format chunk size (16 for PCM)
    WORD audioFormat;       // Audio format (1 for PCM)
    WORD numChannels;       // Number of channels
    DWORD sampleRate;       // Sample rate
    DWORD byteRate;         // Byte rate
    WORD blockAlign;        // Block align
    WORD bitsPerSample;     // Bits per sample
    char data[4];           // "data"
    DWORD dataSize;         // Data size
};
#pragma pack(pop)

// Macro for error handling with critical section unlock
#define RETURN_ERROR(ctx, code) do { \
    LeaveCriticalSection(&(ctx)->cs); \
    return (code); \
} while(0)

struct TTSContext {
    ISpVoice* voice;
    CRITICAL_SECTION cs; // Critical section for thread safety
};

void* tts_create() {
    // Initialize COM with apartment threading
    HRESULT hr = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    if (FAILED(hr) && hr != RPC_E_CHANGED_MODE) {
        return nullptr;
    }

    TTSContext* ctx = new TTSContext();
    ctx->voice = nullptr;
    InitializeCriticalSection(&ctx->cs);

    hr = CoCreateInstance(CLSID_SpVoice, NULL, CLSCTX_ALL, IID_ISpVoice, (void**)&ctx->voice);
    if (FAILED(hr) || !ctx->voice) {
        DeleteCriticalSection(&ctx->cs);
        delete ctx;
        CoUninitialize();
        return nullptr;
    }
    return ctx;
}

void tts_destroy(void* handle) {
    if (handle) {
        TTSContext* ctx = static_cast<TTSContext*>(handle);
        if (ctx->voice) ctx->voice->Release();
        DeleteCriticalSection(&ctx->cs);
        delete ctx;
    }
    CoUninitialize();
}

VoiceInfo* tts_list_voices(void* handle, int* count) {
    if (!handle || !count) return nullptr;
    ISpObjectTokenCategory* category = nullptr;
    IEnumSpObjectTokens* enumTokens = nullptr;
    HRESULT hr = CoCreateInstance(CLSID_SpObjectTokenCategory, NULL, CLSCTX_ALL, IID_ISpObjectTokenCategory, (void**)&category);
    if (FAILED(hr)) return nullptr;
    hr = category->SetId(SPCAT_VOICES, FALSE);
    if (FAILED(hr)) { category->Release(); return nullptr; }
    hr = category->EnumTokens(NULL, NULL, &enumTokens);
    if (FAILED(hr)) { category->Release(); return nullptr; }
    ULONG voiceCount = 0;
    enumTokens->GetCount(&voiceCount);
    *count = voiceCount;
    VoiceInfo* voices = new VoiceInfo[voiceCount];
    for (ULONG i = 0; i < voiceCount; i++) {
        ISpObjectToken* token = nullptr;
        enumTokens->Next(1, &token, NULL);
        LPWSTR id = nullptr, name = nullptr;
        token->GetId(&id);
        SpGetDescription(token, &name);
        int idLen = WideCharToMultiByte(CP_UTF8, 0, id, -1, nullptr, 0, nullptr, nullptr);
        voices[i].id = new char[idLen];
        WideCharToMultiByte(CP_UTF8, 0, id, -1, voices[i].id, idLen, nullptr, nullptr);
        int nameLen = WideCharToMultiByte(CP_UTF8, 0, name, -1, nullptr, 0, nullptr, nullptr);
        voices[i].name = new char[nameLen];
        WideCharToMultiByte(CP_UTF8, 0, name, -1, voices[i].name, nameLen, nullptr, nullptr);
        voices[i].language = new char[6]; strcpy(voices[i].language, "en-US");
        voices[i].gender = new char[7]; strcpy(voices[i].gender, "female");
        CoTaskMemFree(id); CoTaskMemFree(name); token->Release();
    }
    enumTokens->Release(); category->Release();
    return voices;
}

void tts_free_voices(VoiceInfo* voices, int count) {
    if (voices) {
        for (int i = 0; i < count; i++) {
            delete[] voices[i].id; delete[] voices[i].name;
            delete[] voices[i].language; delete[] voices[i].gender;
        }
        delete[] voices;
    }
}

int tts_synthesize(void* handle, const char* text, const char* voice_id, float speed, float pitch, float volume, char** audio_data, int* audio_size, int* sample_rate) {
    if (!handle || !text) return -1;

    TTSContext* ctx = static_cast<TTSContext*>(handle);

    // Lock for thread safety
    EnterCriticalSection(&ctx->cs);

    // Ensure COM is initialized for this thread
    HRESULT hrInit = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    bool needsUninit = SUCCEEDED(hrInit);

    // Set voice if specified
    if (voice_id && strlen(voice_id) > 0) {
        int voiceLen = MultiByteToWideChar(CP_UTF8, 0, voice_id, -1, nullptr, 0);
        wchar_t* wvoice = new wchar_t[voiceLen];
        MultiByteToWideChar(CP_UTF8, 0, voice_id, -1, wvoice, voiceLen);
        ISpObjectToken* token = nullptr;
        HRESULT hr = CoCreateInstance(CLSID_SpObjectToken, NULL, CLSCTX_ALL, __uuidof(ISpObjectToken), (void**)&token);
        if (SUCCEEDED(hr) && token) {
            hr = token->SetId(NULL, wvoice, FALSE);
            if (SUCCEEDED(hr)) {
                ctx->voice->SetVoice(token);
            }
            token->Release();
        }
        delete[] wvoice;
    }

    // Set voice parameters
    long rate = (long)((speed - 1.0f) * 10.0f);
    ctx->voice->SetRate(rate);
    USHORT vol = (USHORT)(volume * 100.0f);
    ctx->voice->SetVolume(vol);

    // Reset output to ensure clean state
    ctx->voice->SetOutput(NULL, TRUE);

    // Convert text to wide string
    int textLen = MultiByteToWideChar(CP_UTF8, 0, text, -1, nullptr, 0);
    wchar_t* wtext = new wchar_t[textLen];
    MultiByteToWideChar(CP_UTF8, 0, text, -1, wtext, textLen);

    // Create memory stream first
    IStream* memStream = nullptr;
    HRESULT hr = CreateStreamOnHGlobal(NULL, TRUE, &memStream);
    if (FAILED(hr) || !memStream) {
        delete[] wtext;
        if (needsUninit) CoUninitialize();
        RETURN_ERROR(ctx, -2);
    }

    // Create SAPI stream wrapper
    ISpStream* stream = nullptr;
    hr = CoCreateInstance(CLSID_SpStream, NULL, CLSCTX_INPROC_SERVER, IID_ISpStream, (void**)&stream);
    if (FAILED(hr) || !stream) {
        memStream->Release();
        delete[] wtext;
        if (needsUninit) CoUninitialize();
        RETURN_ERROR(ctx, -2);
    }

    // Set up wave format (22050 Hz, 16-bit, mono)
    WAVEFORMATEX wfex;
    wfex.wFormatTag = WAVE_FORMAT_PCM;
    wfex.nChannels = 1;
    wfex.nSamplesPerSec = 22050;
    wfex.wBitsPerSample = 16;
    wfex.nBlockAlign = (wfex.nChannels * wfex.wBitsPerSample) / 8;
    wfex.nAvgBytesPerSec = wfex.nSamplesPerSec * wfex.nBlockAlign;
    wfex.cbSize = 0;

    // SetBaseStream with SPDFID_WaveFormatEx should add WAV header
    hr = stream->SetBaseStream(memStream, SPDFID_WaveFormatEx, &wfex);
    if (FAILED(hr)) {
        stream->Release();
        memStream->Release();
        delete[] wtext;
        if (needsUninit) CoUninitialize();
        RETURN_ERROR(ctx, -3);
    }

    // Set output stream - FALSE means we manage the stream lifetime
    hr = ctx->voice->SetOutput(stream, FALSE);
    if (FAILED(hr)) {
        stream->Release();
        memStream->Release();
        delete[] wtext;
        if (needsUninit) CoUninitialize();
        RETURN_ERROR(ctx, -4);
    }

    // Speak the text asynchronously first, then wait
    hr = ctx->voice->Speak(wtext, SPF_ASYNC | SPF_IS_NOT_XML, NULL);
    if (FAILED(hr)) {
        stream->Release();
        memStream->Release();
        delete[] wtext;
        if (needsUninit) CoUninitialize();
        RETURN_ERROR(ctx, -5);
    }

    // Wait for speech to complete
    hr = ctx->voice->WaitUntilDone(INFINITE);
    if (FAILED(hr)) {
        stream->Release();
        memStream->Release();
        delete[] wtext;
        if (needsUninit) CoUninitialize();
        RETURN_ERROR(ctx, -5);
    }

    // Close the stream to flush any pending data
    hr = stream->Close();
    if (FAILED(hr)) {
        stream->Release();
        memStream->Release();
        delete[] wtext;
        if (needsUninit) CoUninitialize();
        RETURN_ERROR(ctx, -5);
    }

    // Get the size of the data in the memory stream
    STATSTG stat;
    memset(&stat, 0, sizeof(stat));
    hr = memStream->Stat(&stat, STATFLAG_NONAME);
    if (FAILED(hr)) {
        stream->Release();
        memStream->Release();
        delete[] wtext;
        if (needsUninit) CoUninitialize();
        RETURN_ERROR(ctx, -6);
    }

    *audio_size = (int)stat.cbSize.QuadPart;

    // Debug: Check if size is actually 0
    if (*audio_size <= 0) {
        // Try to get current position to see if anything was written
        LARGE_INTEGER zero = {0};
        ULARGE_INTEGER currentPos;
        memStream->Seek(zero, STREAM_SEEK_CUR, &currentPos);

        stream->Release();
        memStream->Release();
        delete[] wtext;
        if (needsUninit) CoUninitialize();
        RETURN_ERROR(ctx, -6);
    }

    // Seek to the beginning of the stream
    LARGE_INTEGER pos = {0};
    hr = memStream->Seek(pos, STREAM_SEEK_SET, NULL);
    if (FAILED(hr)) {
        stream->Release();
        memStream->Release();
        delete[] wtext;
        if (needsUninit) CoUninitialize();
        RETURN_ERROR(ctx, -6);
    }

    // Read audio data (PCM data without WAV header from SAPI)
    char* pcmData = new char[*audio_size];
    ULONG bytesRead = 0;
    hr = memStream->Read(pcmData, *audio_size, &bytesRead);
    if (FAILED(hr) || bytesRead != (ULONG)*audio_size) {
        delete[] pcmData;
        stream->Release();
        memStream->Release();
        delete[] wtext;
        if (needsUninit) CoUninitialize();
        RETURN_ERROR(ctx, -7);
    }

    // Create WAV file with header
    int pcmSize = *audio_size;
    int wavSize = sizeof(WAVHeader) + pcmSize;
    *audio_data = new char[wavSize];

    // Fill WAV header
    WAVHeader* header = (WAVHeader*)*audio_data;
    memcpy(header->riff, "RIFF", 4);
    header->fileSize = wavSize - 8;
    memcpy(header->wave, "WAVE", 4);
    memcpy(header->fmt, "fmt ", 4);
    header->fmtSize = 16;
    header->audioFormat = 1; // PCM
    header->numChannels = 1;
    header->sampleRate = 22050;
    header->bitsPerSample = 16;
    header->blockAlign = (header->numChannels * header->bitsPerSample) / 8;
    header->byteRate = header->sampleRate * header->blockAlign;
    memcpy(header->data, "data", 4);
    header->dataSize = pcmSize;

    // Copy PCM data after header
    memcpy(*audio_data + sizeof(WAVHeader), pcmData, pcmSize);
    delete[] pcmData;

    *audio_size = wavSize;
    *sample_rate = 22050;

    stream->Release();
    memStream->Release();
    delete[] wtext;

    // Don't uninitialize COM here as it may be needed by other operations
    // The caller's thread will handle cleanup

    // Unlock
    LeaveCriticalSection(&ctx->cs);

    return 0;
}

void tts_free_audio(char* audio_data) {
    if (audio_data) delete[] audio_data;
}
