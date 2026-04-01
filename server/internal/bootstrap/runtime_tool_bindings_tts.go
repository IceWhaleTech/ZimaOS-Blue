package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
)

func bindRuntimeTTSTool(
	registry *tools.Registry,
	speechSource runtimeSpeechServiceSource,
	voiceSource runtimeVoiceServiceSource,
) {
	if registry == nil || (speechSource == nil && voiceSource == nil) {
		return
	}

	var speechSvc speech.Service
	if speechSource != nil {
		speechSvc = speechSource.Service()
	}

	var voiceSvc voice.Service
	if voiceSource != nil {
		voiceSvc = voiceSource.Service()
	}

	tools.RegisterTTSTool(registry, ttsToolAdapter{speech: speechSvc, voice: voiceSvc})
}
