//go:build darwin

package voicewake

import (
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const (
	speechAuthorizationAuthorized = 3
	avAuthorizationAuthorized     = 3
	audioMediaType                = "soun"
)

var (
	voiceWakePermissionOnce sync.Once

	selAuthorizationStatusForMediaType objc.SEL
)

func initVoiceWakePermissionSelectors() {
	voiceWakePermissionOnce.Do(func() {
		_, _ = purego.Dlopen("/System/Library/Frameworks/AVFoundation.framework/AVFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		selAuthorizationStatusForMediaType = objc.RegisterName("authorizationStatusForMediaType:")
	})
}

func defaultProbePermissionStatus() permissionStatus {
	initVoiceWakeSelectors()
	initVoiceWakePermissionSelectors()

	out := permissionStatus{}

	if speech.CheckSpeechRecognitionAccess() == nil {
		if recognizerClass := objc.ID(objc.GetClass("SFSpeechRecognizer")); recognizerClass != 0 {
			out.SpeechAuthorized = objc.Send[int](recognizerClass, selAuthorizationStatus) == speechAuthorizationAuthorized
		}
	}

	if speech.CheckMicrophoneAccess() == nil {
		if captureClass := objc.ID(objc.GetClass("AVCaptureDevice")); captureClass != 0 {
			out.MicrophoneReady = objc.Send[int](captureClass, selAuthorizationStatusForMediaType, nsString(audioMediaType)) == avAuthorizationAuthorized
		}
	}

	return out
}
