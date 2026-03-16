package voicewake

type permissionStatus struct {
	SpeechAuthorized bool
	MicrophoneReady  bool
}

var probePermissionStatus = defaultProbePermissionStatus
