//go:build darwin

package voicewake

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var (
	voiceWakeOnce sync.Once

	selAlloc                      objc.SEL
	selInit                       objc.SEL
	selInitWithLocale             objc.SEL
	selInitWithLocaleIdentifier   objc.SEL
	selAuthorizationStatus        objc.SEL
	selIsAvailable                objc.SEL
	selRecognitionTaskWithRequest objc.SEL
	selSetShouldReportPartial     objc.SEL
	selBestTranscription          objc.SEL
	selFormattedString            objc.SEL
	selIsFinal                    objc.SEL
	selLocalizedDescription       objc.SEL
	selStringWithUTF8String       objc.SEL
	selUTF8String                 objc.SEL
	selSegments                   objc.SEL
	selCount                      objc.SEL
	selObjectAtIndex              objc.SEL
	selSubstring                  objc.SEL
	selTimestamp                  objc.SEL
	selDuration                   objc.SEL
	selCancel                     objc.SEL
	selInputNode                  objc.SEL
	selOutputFormatForBus         objc.SEL
	selInstallTapOnBus            objc.SEL
	selRemoveTapOnBus             objc.SEL
	selPrepare                    objc.SEL
	selStartAndReturnError        objc.SEL
	selStop                       objc.SEL
	selAppendAudioPCMBuffer       objc.SEL
	selEndAudio                   objc.SEL
)

func initVoiceWakeSelectors() {
	voiceWakeOnce.Do(func() {
		_, _ = purego.Dlopen("/System/Library/Frameworks/Speech.framework/Speech", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		_, _ = purego.Dlopen("/System/Library/Frameworks/AVFoundation.framework/AVFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL)

		selAlloc = objc.RegisterName("alloc")
		selInit = objc.RegisterName("init")
		selInitWithLocale = objc.RegisterName("initWithLocale:")
		selInitWithLocaleIdentifier = objc.RegisterName("initWithLocaleIdentifier:")
		selAuthorizationStatus = objc.RegisterName("authorizationStatus")
		selIsAvailable = objc.RegisterName("isAvailable")
		selRecognitionTaskWithRequest = objc.RegisterName("recognitionTaskWithRequest:resultHandler:")
		selSetShouldReportPartial = objc.RegisterName("setShouldReportPartialResults:")
		selBestTranscription = objc.RegisterName("bestTranscription")
		selFormattedString = objc.RegisterName("formattedString")
		selIsFinal = objc.RegisterName("isFinal")
		selLocalizedDescription = objc.RegisterName("localizedDescription")
		selStringWithUTF8String = objc.RegisterName("stringWithUTF8String:")
		selUTF8String = objc.RegisterName("UTF8String")
		selSegments = objc.RegisterName("segments")
		selCount = objc.RegisterName("count")
		selObjectAtIndex = objc.RegisterName("objectAtIndex:")
		selSubstring = objc.RegisterName("substring")
		selTimestamp = objc.RegisterName("timestamp")
		selDuration = objc.RegisterName("duration")
		selCancel = objc.RegisterName("cancel")
		selInputNode = objc.RegisterName("inputNode")
		selOutputFormatForBus = objc.RegisterName("outputFormatForBus:")
		selInstallTapOnBus = objc.RegisterName("installTapOnBus:bufferSize:format:block:")
		selRemoveTapOnBus = objc.RegisterName("removeTapOnBus:")
		selPrepare = objc.RegisterName("prepare")
		selStartAndReturnError = objc.RegisterName("startAndReturnError:")
		selStop = objc.RegisterName("stop")
		selAppendAudioPCMBuffer = objc.RegisterName("appendAudioPCMBuffer:")
		selEndAudio = objc.RegisterName("endAudio")
	})
}

func nsString(s string) objc.ID {
	cstr := append([]byte(s), 0)
	cls := objc.ID(objc.GetClass("NSString"))
	return cls.Send(selStringWithUTF8String, uintptr(unsafe.Pointer(&cstr[0])))
}

func goString(nsStr objc.ID) string {
	if nsStr == 0 {
		return ""
	}
	ptr := objc.Send[uintptr](nsStr, selUTF8String)
	if ptr == 0 {
		return ""
	}
	length := 0
	for {
		b := *(*byte)(unsafe.Add(unsafe.Pointer(ptr), length))
		if b == 0 {
			break
		}
		length++
	}
	if length == 0 {
		return ""
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), length))
}

type darwinRuntime struct {
	mu                 sync.Mutex
	running            bool
	cfg                RuntimeConfig
	stopCh             chan struct{}
	restartPending     bool
	generation         int
	engine             objc.ID
	inputNode          objc.ID
	request            objc.ID
	task               objc.ID
	recognitionBlock   objc.Block
	tapBlock           objc.Block
	capturing          bool
	captureStartedAt   time.Time
	lastTranscriptAt   time.Time
	triggerEndSeconds  float64
	capturedTranscript string
	cooldownUntil      time.Time
}

type startPipelineResult struct {
	engine           objc.ID
	inputNode        objc.ID
	request          objc.ID
	task             objc.ID
	tapBlock         objc.Block
	recognitionBlock objc.Block
	err              error
}

func newRuntime() runtimeController {
	return &darwinRuntime{}
}

func (r *darwinRuntime) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *darwinRuntime) Start(cfg RuntimeConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.stopLocked()
	r.cfg = cfg
	r.stopCh = make(chan struct{})
	r.running = true
	r.restartPending = false
	r.resetCaptureLocked()
	if err := r.startPipelineLocked(); err != nil {
		r.running = false
		close(r.stopCh)
		r.stopCh = nil
		return err
	}
	go r.watchLoop(r.stopCh)
	return nil
}

func (r *darwinRuntime) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopLocked()
}

func (r *darwinRuntime) stopLocked() error {
	if r.stopCh != nil {
		close(r.stopCh)
		r.stopCh = nil
	}
	r.running = false
	r.restartPending = false
	r.resetCaptureLocked()
	return r.stopPipelineLocked()
}

func (r *darwinRuntime) startPipelineLocked() error {
	cfg := r.cfg
	r.generation++
	generation := r.generation
	resultCh := make(chan startPipelineResult, 1)
	speech.SubmitToMainThread(func() {
		initVoiceWakeSelectors()
		recognizerClass := objc.ID(objc.GetClass("SFSpeechRecognizer"))
		if recognizerClass == 0 {
			resultCh <- startPipelineResult{err: ErrRecognizerUnavailable}
			return
		}
		if status := objc.Send[int](recognizerClass, selAuthorizationStatus); status != 3 {
			resultCh <- startPipelineResult{err: ErrSpeechUnauthorized}
			return
		}

		var recognizer objc.ID
		if locale := strings.TrimSpace(cfg.Locale); locale != "" {
			nsLocaleClass := objc.ID(objc.GetClass("NSLocale"))
			nsLocale := nsLocaleClass.Send(selAlloc).Send(selInitWithLocaleIdentifier, nsString(locale))
			recognizer = recognizerClass.Send(selAlloc).Send(selInitWithLocale, nsLocale)
		} else {
			recognizer = recognizerClass.Send(selAlloc).Send(selInit)
		}
		if recognizer == 0 {
			resultCh <- startPipelineResult{err: ErrRecognizerUnavailable}
			return
		}
		if !objc.Send[bool](recognizer, selIsAvailable) {
			resultCh <- startPipelineResult{err: ErrRecognizerUnavailable}
			return
		}

		requestClass := objc.ID(objc.GetClass("SFSpeechAudioBufferRecognitionRequest"))
		request := requestClass.Send(selAlloc).Send(selInit)
		if request == 0 {
			resultCh <- startPipelineResult{err: fmt.Errorf("failed to create voice wake recognition request")}
			return
		}
		request.Send(selSetShouldReportPartial, true)

		engineClass := objc.ID(objc.GetClass("AVAudioEngine"))
		engine := engineClass.Send(selAlloc).Send(selInit)
		if engine == 0 {
			resultCh <- startPipelineResult{err: ErrMicrophoneUnavailable}
			return
		}
		inputNode := engine.Send(selInputNode)
		if inputNode == 0 {
			resultCh <- startPipelineResult{err: ErrMicrophoneUnavailable}
			return
		}
		format := inputNode.Send(selOutputFormatForBus, uintptr(0))
		if format == 0 {
			resultCh <- startPipelineResult{err: ErrMicrophoneUnavailable}
			return
		}
		inputNode.Send(selRemoveTapOnBus, uintptr(0))

		tapBlock := objc.NewBlock(func(_ objc.Block, buffer objc.ID, _ objc.ID) {
			if buffer != 0 {
				request.Send(selAppendAudioPCMBuffer, buffer)
			}
		})
		inputNode.Send(selInstallTapOnBus, uintptr(0), uintptr(2048), format, tapBlock)

		recognitionBlock := objc.NewBlock(func(_ objc.Block, res objc.ID, nsErr objc.ID) {
			if nsErr != 0 {
				go r.handleUpdate(generation, "", nil, false, mapVoiceWakeError(goString(nsErr.Send(selLocalizedDescription))))
				return
			}
			if res == 0 {
				return
			}
			transcription := res.Send(selBestTranscription)
			text := strings.TrimSpace(goString(transcription.Send(selFormattedString)))
			segments := transcriptionSegments(transcription)
			isFinal := objc.Send[bool](res, selIsFinal)
			go r.handleUpdate(generation, text, segments, isFinal, nil)
		})
		task := recognizer.Send(selRecognitionTaskWithRequest, request, recognitionBlock)
		engine.Send(selPrepare)
		var startErr objc.ID
		started := objc.Send[bool](engine, selStartAndReturnError, unsafe.Pointer(&startErr))
		if !started {
			inputNode.Send(selRemoveTapOnBus, uintptr(0))
			tapBlock.Release()
			recognitionBlock.Release()
			if startErr != 0 {
				resultCh <- startPipelineResult{err: mapVoiceWakeError(goString(startErr.Send(selLocalizedDescription)))}
			} else {
				resultCh <- startPipelineResult{err: ErrMicrophoneUnavailable}
			}
			return
		}

		resultCh <- startPipelineResult{
			engine:           engine,
			inputNode:        inputNode,
			request:          request,
			task:             task,
			tapBlock:         tapBlock,
			recognitionBlock: recognitionBlock,
		}
	})
	result := <-resultCh
	if result.err != nil {
		return result.err
	}
	if generation != r.generation || !r.running {
		if result.inputNode != 0 {
			result.inputNode.Send(selRemoveTapOnBus, uintptr(0))
		}
		if result.engine != 0 {
			result.engine.Send(selStop)
		}
		if result.request != 0 {
			result.request.Send(selEndAudio)
		}
		if result.task != 0 {
			result.task.Send(selCancel)
		}
		if result.tapBlock != 0 {
			result.tapBlock.Release()
		}
		if result.recognitionBlock != 0 {
			result.recognitionBlock.Release()
		}
		return nil
	}
	r.engine = result.engine
	r.inputNode = result.inputNode
	r.request = result.request
	r.task = result.task
	r.tapBlock = result.tapBlock
	r.recognitionBlock = result.recognitionBlock
	return nil
}

func (r *darwinRuntime) stopPipelineLocked() error {
	generation := r.generation
	engine := r.engine
	inputNode := r.inputNode
	request := r.request
	task := r.task
	tapBlock := r.tapBlock
	recognitionBlock := r.recognitionBlock
	r.engine = 0
	r.inputNode = 0
	r.request = 0
	r.task = 0
	r.tapBlock = 0
	r.recognitionBlock = 0
	if generation == 0 && engine == 0 && inputNode == 0 && request == 0 && task == 0 {
		return nil
	}
	done := make(chan struct{}, 1)
	speech.SubmitToMainThread(func() {
		if inputNode != 0 {
			inputNode.Send(selRemoveTapOnBus, uintptr(0))
		}
		if request != 0 {
			request.Send(selEndAudio)
		}
		if task != 0 {
			task.Send(selCancel)
		}
		if engine != 0 {
			engine.Send(selStop)
		}
		if tapBlock != 0 {
			tapBlock.Release()
		}
		if recognitionBlock != 0 {
			recognitionBlock.Release()
		}
		done <- struct{}{}
	})
	<-done
	return nil
}

func (r *darwinRuntime) handleUpdate(generation int, transcript string, segments []Segment, isFinal bool, err error) {
	if err != nil {
		if errorsIsFatalVoiceWake(err) {
			_ = r.Stop()
			if cb := r.cfg.OnError; cb != nil {
				cb(err)
			}
			return
		}
		if cb := r.cfg.OnError; cb != nil {
			cb(err)
		}
		r.scheduleRestart(500 * time.Millisecond)
		return
	}

	var (
		triggerCB      func(GateMatch)
		finalizeText   string
		shouldFinalize bool
		shouldRestart  bool
	)
	r.mu.Lock()
	if generation != r.generation || !r.running {
		r.mu.Unlock()
		return
	}
	now := time.Now()
	if !r.cooldownUntil.IsZero() && now.Before(r.cooldownUntil) {
		r.mu.Unlock()
		return
	}
	if strings.TrimSpace(transcript) != "" {
		if !r.capturing {
			if match := MatchSegments(transcript, segments, GateConfig{Triggers: r.cfg.Triggers, MinPostTriggerGap: r.cfg.MinPostTriggerGap.Seconds(), MinCommandLength: defaultMinCommandLength}); match != nil && match.TriggerFound {
				r.capturing = true
				r.captureStartedAt = now
				r.lastTranscriptAt = now
				r.triggerEndSeconds = match.TriggerEndSeconds
				r.capturedTranscript = strings.TrimSpace(match.Command)
				triggerCB = r.cfg.OnTriggered
			} else if TranscriptContainsWakeWord(transcript, r.cfg.Triggers) {
				r.capturing = true
				r.captureStartedAt = now
				r.lastTranscriptAt = now
				r.capturedTranscript = ""
				triggerCB = r.cfg.OnTriggered
			}
		} else {
			r.lastTranscriptAt = now
			command := strings.TrimSpace(CommandTextAfterTrigger(transcript, segments, r.triggerEndSeconds))
			if command == "" {
				command = strings.TrimSpace(StripWakeWordFromTranscript(transcript, r.cfg.Triggers))
			}
			if command != "" {
				r.capturedTranscript = command
			}
		}
	}
	if isFinal {
		if r.capturing && strings.TrimSpace(r.capturedTranscript) != "" {
			finalizeText = r.capturedTranscript
			shouldFinalize = true
			r.prepareForRestartLocked()
		} else {
			shouldRestart = true
		}
	}
	r.mu.Unlock()

	if triggerCB != nil {
		triggerCB(GateMatch{TriggerFound: true, Command: strings.TrimSpace(finalizeText)})
	}
	if shouldFinalize {
		if cb := r.cfg.OnFinalized; cb != nil {
			cb(strings.TrimSpace(finalizeText))
		}
		r.scheduleRestart(r.cfg.PostSendDebounce)
		return
	}
	if shouldRestart {
		r.scheduleRestart(250 * time.Millisecond)
	}
}

func (r *darwinRuntime) watchLoop(stopCh chan struct{}) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			var (
				finalizeText   string
				shouldFinalize bool
				finalizeCB     func(string)
			)
			r.mu.Lock()
			if !r.running {
				r.mu.Unlock()
				return
			}
			if r.capturing {
				now := time.Now()
				if !r.captureStartedAt.IsZero() && now.Sub(r.captureStartedAt) >= r.cfg.HardStop {
					finalizeText = strings.TrimSpace(r.capturedTranscript)
					shouldFinalize = finalizeText != ""
					finalizeCB = r.cfg.OnFinalized
					r.prepareForRestartLocked()
				} else if !r.lastTranscriptAt.IsZero() {
					if strings.TrimSpace(r.capturedTranscript) != "" && now.Sub(r.lastTranscriptAt) >= r.cfg.PostTriggerSilence {
						finalizeText = strings.TrimSpace(r.capturedTranscript)
						shouldFinalize = true
						finalizeCB = r.cfg.OnFinalized
						r.prepareForRestartLocked()
					} else if strings.TrimSpace(r.capturedTranscript) == "" && now.Sub(r.lastTranscriptAt) >= r.cfg.TriggerOnlySilence {
						r.prepareForRestartLocked()
					}
				}
			}
			r.mu.Unlock()
			if finalizeText != "" && shouldFinalize && finalizeCB != nil {
				finalizeCB(finalizeText)
			}
			if shouldFinalize {
				r.scheduleRestart(r.cfg.PostSendDebounce)
			}
		}
	}
}

func (r *darwinRuntime) scheduleRestart(delay time.Duration) {
	r.mu.Lock()
	if !r.running || r.restartPending || r.stopCh == nil {
		r.mu.Unlock()
		return
	}
	stopCh := r.stopCh
	r.restartPending = true
	r.mu.Unlock()
	go func() {
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-stopCh:
				return
			}
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if !r.running || r.stopCh == nil || stopCh != r.stopCh {
			r.restartPending = false
			return
		}
		_ = r.stopPipelineLocked()
		r.restartPending = false
		if err := r.startPipelineLocked(); err != nil {
			r.running = false
			if r.stopCh != nil {
				close(r.stopCh)
				r.stopCh = nil
			}
			if cb := r.cfg.OnError; cb != nil {
				go cb(err)
			}
		}
	}()
}

func (r *darwinRuntime) prepareForRestartLocked() {
	r.resetCaptureLocked()
	r.cooldownUntil = time.Now().Add(r.cfg.PostSendDebounce)
}

func (r *darwinRuntime) resetCaptureLocked() {
	r.capturing = false
	r.captureStartedAt = time.Time{}
	r.lastTranscriptAt = time.Time{}
	r.triggerEndSeconds = 0
	r.capturedTranscript = ""
}

func transcriptionSegments(transcription objc.ID) []Segment {
	if transcription == 0 {
		return nil
	}
	array := transcription.Send(selSegments)
	if array == 0 {
		return nil
	}
	count := int(objc.Send[uint64](array, selCount))
	if count <= 0 {
		return nil
	}
	segments := make([]Segment, 0, count)
	for i := 0; i < count; i++ {
		segment := array.Send(selObjectAtIndex, uintptr(i))
		if segment == 0 {
			continue
		}
		text := strings.TrimSpace(goString(segment.Send(selSubstring)))
		segments = append(segments, Segment{
			Text:     text,
			Start:    objc.Send[float64](segment, selTimestamp),
			Duration: objc.Send[float64](segment, selDuration),
		})
	}
	return segments
}

func mapVoiceWakeError(desc string) error {
	lower := strings.ToLower(strings.TrimSpace(desc))
	switch {
	case lower == "":
		return fmt.Errorf("voice wake recognition failed")
	case strings.Contains(lower, "not authorized"), strings.Contains(lower, "permission"), strings.Contains(lower, "denied"), strings.Contains(lower, "restricted"):
		return ErrSpeechUnauthorized
	case strings.Contains(lower, "microphone"), strings.Contains(lower, "audio input"), strings.Contains(lower, "input node"), strings.Contains(lower, "record"):
		return ErrMicrophoneUnavailable
	case strings.Contains(lower, "not available"):
		return ErrRecognizerUnavailable
	default:
		return fmt.Errorf("voice wake recognition failed: %s", desc)
	}
}

func errorsIsFatalVoiceWake(err error) bool {
	return err == ErrSpeechUnauthorized || err == ErrMicrophoneUnavailable
}
