#include "speech_asr_windows.h"
#include <windows.h>
#include <sapi.h>
#include <sphelper.h>
#include <comdef.h>
#include <initguid.h>
#include <string>

DEFINE_GUID(CLSID_SpInprocRecognizer, 0x41B89B6B, 0x9399, 0x11D2, 0x96, 0x23, 0x00, 0xC0, 0x4F, 0x8E, 0xE6, 0x28);
DEFINE_GUID(IID_ISpRecognizer, 0xC2B5F241, 0xDAA0, 0x4507, 0x9E, 0x16, 0x5A, 0x1E, 0xAA, 0x2B, 0x7A, 0x5C);
DEFINE_GUID(CLSID_SpStream, 0x715D9C59, 0x4442, 0x11D2, 0x96, 0x05, 0x00, 0xC0, 0x4F, 0x8E, 0xE6, 0x28);
DEFINE_GUID(IID_ISpStream, 0x12E3CCA9, 0x7518, 0x44C5, 0xA5, 0xE7, 0xBA, 0x5A, 0x79, 0xCB, 0x92, 0x9E);

struct ASRContext {
    ISpRecognizer* recognizer;
    ISpRecoContext* context;
    ISpRecoGrammar* grammar;
    bool grammarLoaded; // Track if grammar is already loaded
};

void* asr_create(const char* language) {
    CoInitialize(NULL);
    ASRContext* ctx = new ASRContext();
    ctx->recognizer = nullptr;
    ctx->context = nullptr;
    ctx->grammar = nullptr;
    ctx->grammarLoaded = false;

    HRESULT hr = CoCreateInstance(CLSID_SpInprocRecognizer, NULL, CLSCTX_ALL, IID_ISpRecognizer, (void**)&ctx->recognizer);
    if (FAILED(hr)) { delete ctx; return nullptr; }

    hr = ctx->recognizer->CreateRecoContext(&ctx->context);
    if (FAILED(hr)) { ctx->recognizer->Release(); delete ctx; return nullptr; }

    // Pre-create and load grammar to avoid repeated loading
    hr = ctx->context->CreateGrammar(0, &ctx->grammar);
    if (SUCCEEDED(hr) && ctx->grammar) {
        hr = ctx->grammar->LoadDictation(NULL, SPLO_STATIC);
        if (SUCCEEDED(hr)) {
            ctx->grammarLoaded = true;
        }
    }

    return ctx;
}

void asr_destroy(void* handle) {
    if (handle) {
        ASRContext* ctx = static_cast<ASRContext*>(handle);
        if (ctx->grammar) ctx->grammar->Release();
        if (ctx->context) ctx->context->Release();
        if (ctx->recognizer) ctx->recognizer->Release();
        delete ctx;
    }
    CoUninitialize();
}

int asr_recognize(void* handle, const char* audio_data, int audio_size, char** result_text, float* confidence) {
    if (!handle || !audio_data || !result_text || audio_size <= 0) return -1;

    ASRContext* ctx = static_cast<ASRContext*>(handle);

    // Pre-allocate memory for the stream
    HGLOBAL hMem = GlobalAlloc(GMEM_MOVEABLE, audio_size);
    if (!hMem) return -2;

    void* pMem = GlobalLock(hMem);
    if (!pMem) {
        GlobalFree(hMem);
        return -2;
    }

    // Copy audio data
    memcpy(pMem, audio_data, audio_size);
    GlobalUnlock(hMem);

    // Create stream from memory
    IStream* stream = nullptr;
    HRESULT hr = CreateStreamOnHGlobal(hMem, TRUE, &stream);
    if (FAILED(hr)) {
        GlobalFree(hMem);
        return -2;
    }

    // Create SAPI stream
    ISpStream* spStream = nullptr;
    hr = CoCreateInstance(CLSID_SpStream, NULL, CLSCTX_ALL, IID_ISpStream, (void**)&spStream);
    if (FAILED(hr)) {
        stream->Release();
        return -4;
    }

    // Set up wave format (16kHz, 16-bit, mono - standard for speech recognition)
    WAVEFORMATEX wfex;
    wfex.wFormatTag = WAVE_FORMAT_PCM;
    wfex.nChannels = 1;
    wfex.nSamplesPerSec = 16000;
    wfex.wBitsPerSample = 16;
    wfex.nBlockAlign = wfex.nChannels * wfex.wBitsPerSample / 8;
    wfex.nAvgBytesPerSec = wfex.nSamplesPerSec * wfex.nBlockAlign;
    wfex.cbSize = 0;

    hr = spStream->SetBaseStream(stream, SPDFID_WaveFormatEx, &wfex);
    if (FAILED(hr)) {
        spStream->Release();
        stream->Release();
        return -5;
    }

    // Set input to the stream
    hr = ctx->recognizer->SetInput(spStream, FALSE);
    if (FAILED(hr)) {
        spStream->Release();
        stream->Release();
        return -6;
    }

    // Activate grammar if already loaded, otherwise create and load
    if (ctx->grammarLoaded && ctx->grammar) {
        // Grammar already loaded, just activate it
        hr = ctx->grammar->SetDictationState(SPRS_ACTIVE);
        if (FAILED(hr)) {
            spStream->Release();
            stream->Release();
            return -9;
        }
    } else if (!ctx->grammar) {
        // Grammar not created yet, create and load it
        hr = ctx->context->CreateGrammar(0, &ctx->grammar);
        if (FAILED(hr)) {
            spStream->Release();
            stream->Release();
            return -7;
        }
        hr = ctx->grammar->LoadDictation(NULL, SPLO_STATIC);
        if (FAILED(hr)) {
            spStream->Release();
            stream->Release();
            return -8;
        }
        hr = ctx->grammar->SetDictationState(SPRS_ACTIVE);
        if (FAILED(hr)) {
            spStream->Release();
            stream->Release();
            return -9;
        }
        ctx->grammarLoaded = true;
    }

    // Set interest in recognition events
    hr = ctx->context->SetInterest(SPFEI(SPEI_RECOGNITION) | SPFEI(SPEI_END_SR_STREAM),
                                    SPFEI(SPEI_RECOGNITION) | SPFEI(SPEI_END_SR_STREAM));
    if (FAILED(hr)) {
        spStream->Release();
        stream->Release();
        return -10;
    }

    // Wait for recognition event (optimized: shorter polling interval, timeout based on audio length)
    SPEVENT event;
    bool gotResult = false;
    std::wstring resultText;
    float conf = 0.0f;

    // Calculate timeout based on audio length (audio_size / bytes_per_second + 2 seconds buffer)
    int timeoutMs = (audio_size / (wfex.nAvgBytesPerSec / 1000)) + 2000;
    if (timeoutMs > 30000) timeoutMs = 30000; // Max 30 seconds
    if (timeoutMs < 2000) timeoutMs = 2000;   // Min 2 seconds

    int iterations = timeoutMs / 50; // Check every 50ms instead of 100ms
    for (int i = 0; i < iterations; i++) {
        hr = ctx->context->GetEvents(1, &event, NULL);
        if (hr == S_OK) {
            if (event.eEventId == SPEI_RECOGNITION) {
                ISpRecoResult* result = reinterpret_cast<ISpRecoResult*>(event.lParam);
                if (result) {
                    WCHAR* text = nullptr;
                    hr = result->GetText(SP_GETWHOLEPHRASE, SP_GETWHOLEPHRASE, TRUE, &text, NULL);
                    if (SUCCEEDED(hr) && text) {
                        resultText = text;
                        CoTaskMemFree(text);

                        // Get confidence
                        SPPHRASE* phrase = nullptr;
                        if (SUCCEEDED(result->GetPhrase(&phrase)) && phrase) {
                            if (phrase->pElements && phrase->Rule.ulCountOfElements > 0) {
                                conf = phrase->pElements[0].SREngineConfidence;
                            }
                            CoTaskMemFree(phrase);
                        }
                        gotResult = true;
                    }
                    result->Release();
                }
                break;
            } else if (event.eEventId == SPEI_END_SR_STREAM) {
                // Stream ended without recognition
                break;
            }
        }
        if (gotResult) break;
        Sleep(50); // Reduced from 100ms to 50ms for faster response
    }

    spStream->Release();
    stream->Release();

    if (!gotResult || resultText.empty()) {
        return -11; // No recognition result
    }

    // Convert wstring to UTF-8 (optimized: single allocation)
    int utf8Len = WideCharToMultiByte(CP_UTF8, 0, resultText.c_str(), -1, nullptr, 0, nullptr, nullptr);
    if (utf8Len <= 0) {
        return -12;
    }

    *result_text = new char[utf8Len];
    WideCharToMultiByte(CP_UTF8, 0, resultText.c_str(), -1, *result_text, utf8Len, nullptr, nullptr);
    *confidence = conf;

    return 0;
}

void asr_free_result(char* text) {
    if (text) delete[] text;
}

// Get installed speech recognition languages
char** asr_get_installed_languages(int* count) {
    if (!count) return nullptr;

    *count = 0;
    CoInitialize(NULL);

    ISpObjectTokenCategory* category = nullptr;
    HRESULT hr = CoCreateInstance(CLSID_SpObjectTokenCategory, NULL, CLSCTX_ALL,
                                   IID_ISpObjectTokenCategory, (void**)&category);
    if (FAILED(hr)) {
        CoUninitialize();
        return nullptr;
    }

    // Set category to speech recognizers
    hr = category->SetId(SPCAT_RECOGNIZERS, FALSE);
    if (FAILED(hr)) {
        category->Release();
        CoUninitialize();
        return nullptr;
    }

    IEnumSpObjectTokens* enumTokens = nullptr;
    hr = category->EnumTokens(NULL, NULL, &enumTokens);
    if (FAILED(hr)) {
        category->Release();
        CoUninitialize();
        return nullptr;
    }

    ULONG tokenCount = 0;
    enumTokens->GetCount(&tokenCount);

    if (tokenCount == 0) {
        enumTokens->Release();
        category->Release();
        CoUninitialize();
        return nullptr;
    }

    // Allocate array for language codes
    char** languages = new char*[tokenCount];
    int langIndex = 0;

    for (ULONG i = 0; i < tokenCount; i++) {
        ISpObjectToken* token = nullptr;
        if (SUCCEEDED(enumTokens->Next(1, &token, NULL))) {
            // Get language attribute
            WCHAR* langId = nullptr;
            hr = token->GetStringValue(L"Language", &langId);
            if (SUCCEEDED(hr) && langId) {
                // Convert hex language ID to locale name (e.g., 0x409 -> en-US)
                LCID lcid = wcstoul(langId, nullptr, 16);
                WCHAR localeName[LOCALE_NAME_MAX_LENGTH];
                if (LCIDToLocaleName(lcid, localeName, LOCALE_NAME_MAX_LENGTH, 0) > 0) {
                    // Convert to UTF-8
                    int utf8Len = WideCharToMultiByte(CP_UTF8, 0, localeName, -1, nullptr, 0, nullptr, nullptr);
                    if (utf8Len > 0) {
                        languages[langIndex] = new char[utf8Len];
                        WideCharToMultiByte(CP_UTF8, 0, localeName, -1, languages[langIndex], utf8Len, nullptr, nullptr);
                        langIndex++;
                    }
                }
                CoTaskMemFree(langId);
            }
            token->Release();
        }
    }

    enumTokens->Release();
    category->Release();
    CoUninitialize();

    *count = langIndex;
    if (langIndex == 0) {
        delete[] languages;
        return nullptr;
    }

    return languages;
}

void asr_free_languages(char** languages, int count) {
    if (languages) {
        for (int i = 0; i < count; i++) {
            if (languages[i]) delete[] languages[i];
        }
        delete[] languages;
    }
}
