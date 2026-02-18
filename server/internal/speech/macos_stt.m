#import <Speech/Speech.h>
#import <Foundation/Foundation.h>
#import <AVFoundation/AVFoundation.h>

typedef struct {
    void* recognizer;
    void* audioEngine;
} macos_stt_t;

// Speech recognition delegate
@interface SpeechRecognitionDelegate : NSObject <SFSpeechRecognitionTaskDelegate>
@property (nonatomic, strong) NSString* recognizedText;
@property (nonatomic, assign) BOOL completed;
@property (nonatomic, assign) BOOL failed;
@end

@implementation SpeechRecognitionDelegate
- (instancetype)init {
    self = [super init];
    if (self) {
        self.recognizedText = @"";
        self.completed = NO;
        self.failed = NO;
    }
    return self;
}

- (void)speechRecognitionTask:(SFSpeechRecognitionTask *)task didFinishRecognition:(SFSpeechRecognitionResult *)result {
    if (result.isFinal) {
        self.recognizedText = result.bestTranscription.formattedString;
        self.completed = YES;
    }
}

- (void)speechRecognitionTask:(SFSpeechRecognitionTask *)task didFinishSuccessfully:(BOOL)successfully {
    if (!successfully) {
        self.failed = YES;
    }
}

- (void)speechRecognitionTaskWasCancelled:(SFSpeechRecognitionTask *)task {
    self.failed = YES;
}

- (void)speechRecognitionTask:(SFSpeechRecognitionTask *)task didHypothesizeTranscription:(SFTranscriptionSegment *)transcription {
    // Called during recognition
}
@end

// Initialize STT
macos_stt_t* macos_stt_init(void) {
    macos_stt_t* stt = malloc(sizeof(macos_stt_t));
    if (!stt) return NULL;

    SFSpeechRecognizer* recognizer = [[SFSpeechRecognizer alloc] initWithLocale:[NSLocale localeWithLocaleIdentifier:@"en-US"]];
    if (!recognizer) {
        free(stt);
        return NULL;
    }

    stt->recognizer = (void*)CFBridgingRetain(recognizer);
    stt->audioEngine = NULL;
    return stt;
}

// Recognize audio from file
char* macos_stt_recognize(macos_stt_t* stt, const char* audioPath, int* error_code) {
    if (!stt || !audioPath || !error_code) {
        if (error_code) *error_code = 1;
        return NULL;
    }

    SFSpeechRecognizer* recognizer = (__bridge SFSpeechRecognizer*)stt->recognizer;
    if (!recognizer) {
        *error_code = 2;
        return NULL;
    }

    // Check authorization
    if ([SFSpeechRecognizer authorizationStatus] != SFSpeechRecognizerAuthorizationStatusAuthorized) {
        *error_code = 3;
        return NULL;
    }

    NSString* pathStr = [NSString stringWithUTF8String:audioPath];
    NSURL* audioURL = [NSURL fileURLWithPath:pathStr];

    NSError* error = nil;
    AVAudioFile* audioFile = [[AVAudioFile alloc] initForReading:audioURL error:&error];
    if (!audioFile || error) {
        *error_code = 4;
        return NULL;
    }

    SFSpeechAudioBufferRecognitionRequest* request = [[SFSpeechAudioBufferRecognitionRequest alloc] init];
    request.shouldReportPartialResults = YES;

    SpeechRecognitionDelegate* delegate = [[SpeechRecognitionDelegate alloc] init];

    SFSpeechRecognitionTask* task = [recognizer recognitionTaskWithRequest:request delegate:delegate];

    // Wait for recognition to complete (with timeout)
    NSDate* timeout = [NSDate dateWithTimeIntervalSinceNow:60.0];
    while (!delegate.completed && !delegate.failed && [[NSDate date] compare:timeout] == NSOrderedAscending) {
        [[NSRunLoop currentRunLoop] runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.1]];
    }

    if (delegate.failed || !delegate.completed) {
        *error_code = 5;
        return NULL;
    }

    const char* result = [delegate.recognizedText UTF8String];
    char* resultCopy = malloc(strlen(result) + 1);
    if (resultCopy) {
        strcpy(resultCopy, result);
    }

    *error_code = 0;
    return resultCopy;
}

// Cleanup
void macos_stt_cleanup(macos_stt_t* stt) {
    if (!stt) return;

    if (stt->recognizer) {
        SFSpeechRecognizer* recognizer = (SFSpeechRecognizer*)CFBridgingRelease(stt->recognizer);
        (void)recognizer;
    }

    free(stt);
}

void macos_stt_free_string(char* str) {
    if (str) free(str);
}
