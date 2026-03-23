package videogen

import (
	"context"
	"time"
)

// HelperStatus reports whether the native video helper is available.
type HelperStatus struct {
	Available bool   `json:"available"`
	Mode      string `json:"mode,omitempty"`
	Path      string `json:"path,omitempty"`
	Error     string `json:"error,omitempty"`
}

// FrameRect describes a layer frame in output pixel space.
type FrameRect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// LayerSpec defines an animatable visual layer for native video rendering.
type LayerSpec struct {
	ID           string    `json:"id,omitempty"`
	ImagePath    string    `json:"image_path"`
	ContentMode  string    `json:"content_mode,omitempty"`
	ZIndex       int       `json:"z_index,omitempty"`
	StartSec     float64   `json:"start_sec,omitempty"`
	EndSec       float64   `json:"end_sec,omitempty"`
	FrameStart   FrameRect `json:"frame_start"`
	FrameEnd     FrameRect `json:"frame_end"`
	OpacityStart float64   `json:"opacity_start,omitempty"`
	OpacityEnd   float64   `json:"opacity_end,omitempty"`
}

// Job is the serialized request passed to the macOS native video helper.
type Job struct {
	Width         int         `json:"width"`
	Height        int         `json:"height"`
	FPS           int         `json:"fps"`
	DurationSec   int         `json:"duration_sec"`
	Layers        []LayerSpec `json:"layers"`
	AudioPath     string      `json:"audio_path,omitempty"`
	OutputPath    string      `json:"output_path"`
	ThumbnailPath string      `json:"thumbnail_path"`
	StatusPath    string      `json:"status_path"`
	ThumbnailSec  float64     `json:"thumbnail_sec,omitempty"`
}

// Progress is a coarse-grained helper progress snapshot.
type Progress struct {
	Stage     string    `json:"stage,omitempty"`
	Progress  float64   `json:"progress,omitempty"`
	Message   string    `json:"message,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// Result describes the generated native video outputs.
type Result struct {
	OutputPath    string `json:"output_path"`
	ThumbnailPath string `json:"thumbnail_path"`
	DurationSec   int    `json:"duration_sec,omitempty"`
}

// ProgressObserver receives helper progress updates.
type ProgressObserver func(Progress)

// Generator produces local timeline-rendered videos.
type Generator interface {
	Status() HelperStatus
	Available() bool
	Generate(
		ctx context.Context,
		job *Job,
		pollInterval time.Duration,
		stallTimeout time.Duration,
		maxRuntime time.Duration,
		observer ProgressObserver,
	) (*Result, error)
}
