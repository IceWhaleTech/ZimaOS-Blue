package convert

import (
	"encoding/json"
	"time"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusProcessing TaskStatus = "processing"
	StatusSucceeded  TaskStatus = "succeeded"
	StatusFailed     TaskStatus = "failed"
	StatusCancelled  TaskStatus = "cancelled"
)

func (s TaskStatus) IsTerminal() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

const (
	ActionConvert      = "convert"
	ActionMerge        = "merge"
	ActionSplit        = "split"
	ActionTrim         = "trim"
	ActionExtractAudio = "extract_audio"
	ActionExtractFrame = "extract_frames"
	ActionTTS          = "tts"
	ActionASR          = "asr"
)

type OutputPreviewKind string

const (
	PreviewFile  OutputPreviewKind = "file"
	PreviewAudio OutputPreviewKind = "audio"
	PreviewVideo OutputPreviewKind = "video"
	PreviewImage OutputPreviewKind = "image"
	PreviewPDF   OutputPreviewKind = "pdf"
	PreviewText  OutputPreviewKind = "text"
)

type TimeSegment struct {
	StartMS int64 `json:"start_ms,omitempty"`
	EndMS   int64 `json:"end_ms,omitempty"`
}

type DocumentOptions struct {
	Pages []int `json:"pages,omitempty"`
}

type ImageOptions struct {
	Quality int `json:"quality,omitempty"`
	Width   int `json:"width,omitempty"`
	Height  int `json:"height,omitempty"`
}

type AudioOptions struct {
	BitRate    int    `json:"bit_rate,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
	Codec      string `json:"codec,omitempty"`
}

type VideoOptions struct {
	Codec           string        `json:"codec,omitempty"`
	Width           int           `json:"width,omitempty"`
	Height          int           `json:"height,omitempty"`
	FrameRate       int           `json:"frame_rate,omitempty"`
	BitRate         int           `json:"bit_rate,omitempty"`
	StartMS         int64         `json:"start_ms,omitempty"`
	EndMS           int64         `json:"end_ms,omitempty"`
	FrameIntervalMS int64         `json:"frame_interval_ms,omitempty"`
	Segments        []TimeSegment `json:"segments,omitempty"`
}

type TTSSpeechOptions struct {
	Voice  string `json:"voice,omitempty"`
	Rate   int    `json:"rate,omitempty"`
	Format string `json:"format,omitempty"`
}

type ASRSpeechOptions struct {
	Language     string `json:"language,omitempty"`
	OnDeviceOnly bool   `json:"on_device_only,omitempty"`
}

type SpeechOptions struct {
	TTS TTSSpeechOptions `json:"tts,omitempty"`
	ASR ASRSpeechOptions `json:"asr,omitempty"`
}

type TaskOptions struct {
	Document DocumentOptions `json:"document,omitempty"`
	Image    ImageOptions    `json:"image,omitempty"`
	Audio    AudioOptions    `json:"audio,omitempty"`
	Video    VideoOptions    `json:"video,omitempty"`
	Speech   SpeechOptions   `json:"speech,omitempty"`
}

type TaskRequest struct {
	Action       string      `json:"action"`
	Sources      []string    `json:"sources,omitempty"`
	TargetFormat string      `json:"target_format,omitempty"`
	Text         string      `json:"text,omitempty"`
	TaskID       string      `json:"task_id,omitempty"`
	WaitMS       int         `json:"wait_ms,omitempty"`
	Options      TaskOptions `json:"options,omitempty"`
}

type ConvertOutput struct {
	ID          string            `json:"output_id"`
	Name        string            `json:"name"`
	MimeType    string            `json:"mime_type,omitempty"`
	SizeBytes   int64             `json:"size_bytes,omitempty"`
	PreviewKind OutputPreviewKind `json:"preview_kind,omitempty"`
	DownloadURL string            `json:"download_url,omitempty"`
	Ref         string            `json:"ref,omitempty"`
	PreviewText string            `json:"preview_text,omitempty"`
	Path        string            `json:"-"`
}

type ConvertTask struct {
	ID                string          `json:"task_id"`
	ConversationID    string          `json:"conversation_id,omitempty"`
	UserID            string          `json:"user_id,omitempty"`
	Action            string          `json:"action"`
	Status            TaskStatus      `json:"status"`
	Sources           []string        `json:"sources,omitempty"`
	TargetFormat      string          `json:"target_format,omitempty"`
	SourceSummary     string          `json:"source_summary,omitempty"`
	Progress          float64         `json:"progress,omitempty"`
	Message           string          `json:"message,omitempty"`
	Error             string          `json:"error,omitempty"`
	TranscriptPreview string          `json:"transcript_preview,omitempty"`
	Outputs           []ConvertOutput `json:"outputs,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	StartedAt         *time.Time      `json:"started_at,omitempty"`
	CompletedAt       *time.Time      `json:"completed_at,omitempty"`
	Request           *TaskRequest    `json:"-"`
}

type StoredSource struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	ConversationID string    `json:"conversation_id"`
	Name           string    `json:"name"`
	MimeType       string    `json:"mime_type,omitempty"`
	Path           string    `json:"path"`
	SizeBytes      int64     `json:"size_bytes,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type ResolvedSource struct {
	Ref      string `json:"ref"`
	Name     string `json:"name"`
	MimeType string `json:"mime_type,omitempty"`
	Path     string `json:"path"`
	Size     int64  `json:"size_bytes,omitempty"`
	Category string `json:"category,omitempty"`
}

type Capabilities struct {
	Available       bool                   `json:"available"`
	Platform        string                 `json:"platform"`
	Actions         []string               `json:"actions"`
	Tools           map[string]bool        `json:"tools,omitempty"`
	Helper          HelperStatus           `json:"helper"`
	Document        map[string][]string    `json:"document,omitempty"`
	DocumentEngines []DocumentEngineInfo   `json:"document_engines,omitempty"`
	Image           map[string][]string    `json:"image,omitempty"`
	Audio           map[string][]string    `json:"audio,omitempty"`
	Video           map[string]interface{} `json:"video,omitempty"`
	Speech          map[string]interface{} `json:"speech,omitempty"`
	Retention       string                 `json:"retention,omitempty"`
	Notes           []string               `json:"notes,omitempty"`
	WorkingDir      string                 `json:"working_dir,omitempty"`
}

type HelperStatus struct {
	Available bool   `json:"available"`
	Mode      string `json:"mode,omitempty"`
	Path      string `json:"path,omitempty"`
	Error     string `json:"error,omitempty"`
}

type DocumentEngineInfo struct {
	ID        string   `json:"id"`
	Path      string   `json:"path,omitempty"`
	Available bool     `json:"available"`
	Priority  int      `json:"priority"`
	Formats   []string `json:"formats,omitempty"`
}

type helperResponse struct {
	Outputs []helperOutput `json:"outputs,omitempty"`
	Message string         `json:"message,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type helperOutput struct {
	ID          string            `json:"output_id,omitempty"`
	Name        string            `json:"name,omitempty"`
	MimeType    string            `json:"mime_type,omitempty"`
	PreviewKind OutputPreviewKind `json:"preview_kind,omitempty"`
	PreviewText string            `json:"preview_text,omitempty"`
	Path        string            `json:"path,omitempty"`
}

type storedOutput struct {
	ID          string            `json:"output_id"`
	Name        string            `json:"name"`
	MimeType    string            `json:"mime_type,omitempty"`
	SizeBytes   int64             `json:"size_bytes,omitempty"`
	PreviewKind OutputPreviewKind `json:"preview_kind,omitempty"`
	Ref         string            `json:"ref,omitempty"`
	PreviewText string            `json:"preview_text,omitempty"`
	Path        string            `json:"path"`
}

func marshalStoredOutputs(outputs []ConvertOutput) (string, error) {
	encoded := make([]storedOutput, 0, len(outputs))
	for _, output := range outputs {
		encoded = append(encoded, storedOutput{
			ID:          output.ID,
			Name:        output.Name,
			MimeType:    output.MimeType,
			SizeBytes:   output.SizeBytes,
			PreviewKind: output.PreviewKind,
			Ref:         output.Ref,
			PreviewText: output.PreviewText,
			Path:        output.Path,
		})
	}
	data, err := json.Marshal(encoded)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalStoredOutputs(raw string) ([]ConvertOutput, error) {
	trimmed := raw
	if trimmed == "" {
		return nil, nil
	}
	var encoded []storedOutput
	if err := json.Unmarshal([]byte(trimmed), &encoded); err != nil {
		return nil, err
	}
	outputs := make([]ConvertOutput, 0, len(encoded))
	for _, output := range encoded {
		outputs = append(outputs, ConvertOutput{
			ID:          output.ID,
			Name:        output.Name,
			MimeType:    output.MimeType,
			SizeBytes:   output.SizeBytes,
			PreviewKind: output.PreviewKind,
			Ref:         output.Ref,
			PreviewText: output.PreviewText,
			Path:        output.Path,
		})
	}
	return outputs, nil
}
