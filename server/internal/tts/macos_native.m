#import <AVFoundation/AVFoundation.h>
#import <Foundation/Foundation.h>

typedef struct {
    void* synthesizer;
    void* audioEngine;
    void* audioBuffer;
    int audioBufferSize;
} macos_tts_t;

// Audio delegate to capture synthesized audio
@interface AudioCaptureDelegate : NSObject <AVSpeechSynthesizerDelegate>
@property (nonatomic, strong) NSMutableData* audioData;
@property (nonatomic, assign) BOOL completed;
@property (nonatomic, assign) BOOL failed;
@end

@implementation AudioCaptureDelegate
- (instancetype)init {
    self = [super init];
    if (self) {
        self.audioData = [[NSMutableData alloc] init];
        self.completed = NO;
        self.failed = NO;
    }
    return self;
}

- (void)speechSynthesizer:(AVSpeechSynthesizer *)synthesizer didFinishSpeechUtterance:(AVSpeechUtterance *)utterance {
    self.completed = YES;
}

- (void)speechSynthesizer:(AVSpeechSynthesizer *)synthesizer didCancelSpeechUtterance:(AVSpeechUtterance *)utterance {
    self.failed = YES;
}

- (void)speechSynthesizer:(AVSpeechSynthesizer *)synthesizer willSpeakRangeOfSpeechString:(NSRange)characterRange utterance:(AVSpeechUtterance *)utterance {
    // Called during synthesis
}
@end

// Initialize TTS
macos_tts_t* macos_tts_init(void) {
    macos_tts_t* tts = malloc(sizeof(macos_tts_t));
    if (!tts) return NULL;

    AVSpeechSynthesizer* synthesizer = [[AVSpeechSynthesizer alloc] init];
    if (!synthesizer) {
        free(tts);
        return NULL;
    }

    tts->synthesizer = (void*)CFBridgingRetain(synthesizer);
    tts->audioEngine = NULL;
    tts->audioBuffer = NULL;
    tts->audioBufferSize = 0;
    return tts;
}

// Synthesize text to audio
unsigned char* macos_tts_synthesize(macos_tts_t* tts, const char* text, const char* voice, float rate, float pitch, float volume, int* output_len) {
    if (!tts || !text || !output_len) return NULL;

    AVSpeechSynthesizer* synthesizer = (__bridge AVSpeechSynthesizer*)tts->synthesizer;

    if (!synthesizer) {
        *output_len = 0;
        return NULL;
    }

    NSString* textStr = [NSString stringWithUTF8String:text];
    NSString* voiceStr = [NSString stringWithUTF8String:voice];

    AVSpeechUtterance* utterance = [[AVSpeechUtterance alloc] initWithString:textStr];
    if (!utterance) {
        *output_len = 0;
        return NULL;
    }

    // Set voice
    AVSpeechSynthesisVoice* speechVoice = [AVSpeechSynthesisVoice voiceWithIdentifier:voiceStr];
    if (speechVoice) {
        utterance.voice = speechVoice;
    } else {
        NSArray* voices = [AVSpeechSynthesisVoice speechVoices];
        if ([voices count] > 0) {
            utterance.voice = voices[0];
        }
    }

    utterance.rate = fmax(0.5f, fmin(2.0f, rate));
    utterance.pitchMultiplier = fmax(0.5f, fmin(2.0f, pitch));
    utterance.volume = fmax(0.0f, fmin(1.0f, volume / 100.0f));

    AudioCaptureDelegate* delegate = [[AudioCaptureDelegate alloc] init];
    synthesizer.delegate = delegate;

    [synthesizer speakUtterance:utterance];

    NSDate* timeout = [NSDate dateWithTimeIntervalSinceNow:30.0];
    while (!delegate.completed && !delegate.failed && [[NSDate date] compare:timeout] == NSOrderedAscending) {
        [[NSRunLoop currentRunLoop] runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.1]];
    }

    if (delegate.failed || !delegate.completed) {
        *output_len = 0;
        return NULL;
    }

    // Return minimal WAV header for now (full audio capture requires AVAudioEngine)
    unsigned char* wavHeader = malloc(44);
    if (wavHeader) {
        // Minimal WAV header (44 bytes)
        memcpy(wavHeader, "RIFF", 4);
        memcpy(wavHeader + 8, "WAVE", 4);
        memcpy(wavHeader + 12, "fmt ", 4);
        *output_len = 44;
    } else {
        *output_len = 0;
    }
    return wavHeader;
}

// Get available voices
const char** macos_tts_get_voices(int* count) {
    if (!count) return NULL;

    NSArray* voices = [AVSpeechSynthesisVoice speechVoices];
    *count = (int)[voices count];

    if (*count == 0) return NULL;

    const char** result = malloc(sizeof(char*) * (*count));
    if (!result) return NULL;

    for (int i = 0; i < *count; i++) {
        AVSpeechSynthesisVoice* voice = voices[i];
        const char* identifier = [voice.identifier UTF8String];
        result[i] = strdup(identifier);
    }

    return result;
}

// Cleanup
void macos_tts_cleanup(macos_tts_t* tts) {
    if (!tts) return;

    if (tts->synthesizer) {
        AVSpeechSynthesizer* synthesizer = (AVSpeechSynthesizer*)CFBridgingRelease(tts->synthesizer);
        (void)synthesizer;
    }

    if (tts->audioBuffer) {
        free(tts->audioBuffer);
    }

    free(tts);
}

void macos_tts_free_audio(unsigned char* audio) {
    if (audio) free(audio);
}

void macos_tts_free_voices(const char** voices, int count) {
    if (!voices) return;
    for (int i = 0; i < count; i++) {
        if (voices[i]) free((void*)voices[i]);
    }
    free(voices);
}


