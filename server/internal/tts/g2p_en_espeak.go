//go:build espeak && !windows

package tts

/*
#cgo CFLAGS: -I${SRCDIR}/../../../third_party/espeak-ng/src/include
#cgo darwin LDFLAGS: -L${SRCDIR}/../../../third_party/espeak-ng/build/src/libespeak-ng -lespeak-ng -L${SRCDIR}/../../../third_party/espeak-ng/build/src/ucd-tools -lucd -L${SRCDIR}/../../../third_party/espeak-ng/build/src/speechPlayer -lspeechPlayer -L${SRCDIR}/../../../third_party/espeak-ng/build -lsonic -lc++
#cgo linux LDFLAGS: -L${SRCDIR}/../../../third_party/espeak-ng/build/src/libespeak-ng -lespeak-ng -L${SRCDIR}/../../../third_party/espeak-ng/build/src/ucd-tools -lucd -L${SRCDIR}/../../../third_party/espeak-ng/build/src/speechPlayer -lspeechPlayer -L${SRCDIR}/../../../third_party/espeak-ng/build -lsonic -lstdc++ -lpthread

#include <stdlib.h>
#include <string.h>
#include <espeak-ng/speak_lib.h>

static char* g2p_text_to_phonemes(const char* text, const char* voice) {
    if (espeak_SetVoiceByName(voice) != EE_OK) {
        return NULL;
    }
    const char* input = text;
    int textmode = espeakCHARS_UTF8;
    int phonememode = 0x02; // IPA

    char result[8192];
    int pos = 0;
    result[0] = '\0';

    while (input != NULL && *input != '\0') {
        const char* ph = espeak_TextToPhonemes((const void**)&input, textmode, phonememode);
        if (ph != NULL && *ph != '\0') {
            int len = strlen(ph);
            if (pos + len + 1 < (int)sizeof(result)) {
                if (pos > 0) result[pos++] = ' ';
                memcpy(result + pos, ph, len);
                pos += len;
            }
        }
    }
    result[pos] = '\0';
    char* out = (char*)malloc(pos + 1);
    if (out) memcpy(out, result, pos + 1);
    return out;
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

// espeakFallback converts a word to IPA using eSpeak-NG, then applies from_espeak conversion.
func espeakFallback(word string) string {
	cText := C.CString(word)
	defer C.free(unsafe.Pointer(cText))
	cVoice := C.CString("en")
	defer C.free(unsafe.Pointer(cVoice))

	cResult := C.g2p_text_to_phonemes(cText, cVoice)
	if cResult == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cResult))

	raw := C.GoString(cResult)
	return fromEspeak(strings.TrimSpace(raw))
}
