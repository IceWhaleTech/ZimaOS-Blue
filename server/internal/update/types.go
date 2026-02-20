package update

import "time"

// UpdateInfo represents available update information
type UpdateInfo struct {
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version"`
	UpdateAvailable bool      `json:"update_available"`
	ReleaseChannel  string    `json:"release_channel"`
	ReleaseNotes    string    `json:"release_notes"`
	DownloadURL     string    `json:"download_url"`
	Checksum        string    `json:"checksum"`
	Size            int64     `json:"size"`
	PublishedAt     time.Time `json:"published_at"`
}

// UpdateStatus represents current update status
type UpdateStatus struct {
	State          string    `json:"state"` // idle, checking, downloading, applying, restarting, failed
	Progress       float64   `json:"progress"`
	Error          string    `json:"error,omitempty"`
	LastChecked    time.Time `json:"last_checked"`
	DownloadedPath string    `json:"downloaded_path,omitempty"`
}

// UpdateHistory represents update history entry
type UpdateHistory struct {
	ID          string    `json:"id"`
	FromVersion string    `json:"from_version"`
	ToVersion   string    `json:"to_version"`
	Status      string    `json:"status"` // success, failed, rolled_back
	AppliedAt   time.Time `json:"applied_at"`
	RolledBack  bool      `json:"rolled_back"`
}

// ReleaseInfo represents a release from GitHub
type ReleaseInfo struct {
	Version     string    `json:"version"`
	Channel     string    `json:"channel"` // stable, beta, alpha
	DownloadURL string    `json:"download_url"`
	Checksum    string    `json:"checksum"`
	Size        int64     `json:"size"`
	Notes       string    `json:"notes"`
	PublishedAt time.Time `json:"published_at"`
}

// Update state constants
const (
	StateIdle        = "idle"
	StateChecking    = "checking"
	StateDownloading = "downloading"
	StateApplying    = "applying"
	StateRestarting  = "restarting"
	StateFailed      = "failed"
)

// History status constants
const (
	StatusSuccess    = "success"
	StatusFailed     = "failed"
	StatusRolledBack = "rolled_back"
)

// Release channel constants
const (
	ChannelStable = "stable"
	ChannelBeta   = "beta"
	ChannelAlpha  = "alpha"
)
