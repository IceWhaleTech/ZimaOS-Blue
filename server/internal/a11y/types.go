package a11y

import "context"

const DefaultHoldMS = 600

const (
	ActionCapabilities        = "capabilities"
	ActionWindows             = "windows"
	ActionFocus               = "focus"
	ActionSnapshot            = "snapshot"
	ActionSnapshotInteractive = "snapshot_interactive"
	ActionAct                 = "act"
	ActionScroll              = "scroll"
	ActionPointerMove         = "pointer_move"
	ActionKey                 = "key"
	ActionScreenshot          = "screenshot"
)

var SupportedActions = []string{
	ActionCapabilities,
	ActionWindows,
	ActionFocus,
	ActionSnapshot,
	ActionSnapshotInteractive,
	ActionAct,
	ActionScroll,
	ActionPointerMove,
	ActionKey,
	ActionScreenshot,
}

type PermissionStatus struct {
	Name     string `json:"name"`
	Granted  bool   `json:"granted"`
	Required bool   `json:"required"`
	Message  string `json:"message,omitempty"`
}

type CapabilitiesResult struct {
	HostOS             string             `json:"host_os"`
	Permissions        []PermissionStatus `json:"permissions,omitempty"`
	SupportedActions   []string           `json:"supported_actions,omitempty"`
	UnsupportedActions []string           `json:"unsupported_actions,omitempty"`
	Message            string             `json:"message,omitempty"`
}

type WindowInfo struct {
	ID      string `json:"id"`
	Title   string `json:"title,omitempty"`
	AppName string `json:"app_name,omitempty"`
	PID     int    `json:"pid,omitempty"`
	Focused bool   `json:"focused,omitempty"`
}

type SnapshotResult struct {
	HostOS    string         `json:"host_os"`
	WindowID  string         `json:"window_id"`
	Title     string         `json:"title,omitempty"`
	Tree      string         `json:"tree,omitempty"`
	RefMap    map[int]string `json:"ref_map,omitempty"`
	ImagePath string         `json:"image_path,omitempty"`
	Message   string         `json:"message,omitempty"`
}

type ActionResult struct {
	HostOS             string   `json:"host_os,omitempty"`
	WindowID           string   `json:"window_id,omitempty"`
	ExecutionMode      string   `json:"execution_mode,omitempty"`
	Intent             string   `json:"intent,omitempty"`
	TargetHit          bool     `json:"target_hit,omitempty"`
	VerificationPassed bool     `json:"verification_passed,omitempty"`
	VerificationMethod string   `json:"verification_method,omitempty"`
	InputMethod        string   `json:"input_method,omitempty"`
	Fallbacks          []string `json:"fallbacks,omitempty"`
	OverlayMode        string   `json:"overlay_mode,omitempty"`
	Message            string   `json:"message,omitempty"`
}

type ScreenshotResult struct {
	HostOS    string `json:"host_os,omitempty"`
	WindowID  string `json:"window_id,omitempty"`
	ImagePath string `json:"image_path,omitempty"`
	Message   string `json:"message,omitempty"`
}

type RuntimeError struct {
	Code    string
	Message string
	Details map[string]interface{}
}

func (e *RuntimeError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func NewError(code string, message string, details map[string]interface{}) *RuntimeError {
	return &RuntimeError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

type Backend interface {
	HostOS() string
	Capabilities(ctx context.Context) (CapabilitiesResult, error)
	ListWindows(ctx context.Context) ([]WindowInfo, error)
	FocusWindow(ctx context.Context, windowID string) (ActionResult, error)
	Snapshot(ctx context.Context, windowID string) (SnapshotResult, error)
	SnapshotInteractive(ctx context.Context, windowID string) (SnapshotResult, error)
	Act(ctx context.Context, windowID string, ref int, refMap map[int]string, actType string, value string, holdMS int) (ActionResult, error)
	Scroll(ctx context.Context, windowID string, direction string, lines int) (ActionResult, error)
	PointerMove(ctx context.Context, x int, y int) (ActionResult, error)
	Key(ctx context.Context, windowID string, keys []string, holdMS int) (ActionResult, error)
	Screenshot(ctx context.Context, windowID string) (ScreenshotResult, error)
}
