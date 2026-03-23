package providerpool

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
	"go.uber.org/zap"
)

// remotePricingURL points to the canonical pricing JSON in the repo.
// ExpandGitHubURL will generate jsdelivr + raw.githubusercontent mirrors.
const remotePricingURL = "https://raw.githubusercontent.com/IceWhaleTech/ZimaOS-Blue/main/docs/model_pricing.json"

// remotePricingJSON is the JSON schema for the remote pricing file.
type remotePricingJSON struct {
	Models      map[string]remotePricingItem      `json:"models"`
	MediaModels map[string]remoteMediaPricingItem `json:"media_models"`
}

type remotePricingItem struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
	Cache  float64 `json:"cache"`
}

type remoteMediaPricingItem struct {
	Output float64 `json:"output"`
	Unit   string  `json:"unit"` // "image", "second", "video"
}

// MediaPricingApplier is a callback for applying media model pricing updates.
// The callback receives (modelID, outputPrice, unit) for each media model.
type MediaPricingApplier func(modelID string, outputPrice float64, unit string)

// PricingUpdater periodically fetches remote pricing and merges into BuiltinModelPricing.
// Uses HTTP ETag to avoid re-parsing unchanged content.
type PricingUpdater struct {
	dl                  *downloader.Downloader
	logger              *zap.Logger
	interval            time.Duration
	stopCh              chan struct{}
	fetchMu             sync.Mutex
	mu                  sync.RWMutex
	lastETag            string // tracks whether content actually changed
	cachedContent       string
	mediaPricingApplier MediaPricingApplier
}

// NewPricingUpdater creates a new updater. Call Start() to begin background polling.
func NewPricingUpdater(logger *zap.Logger) *PricingUpdater {
	return &PricingUpdater{
		dl: &downloader.Downloader{
			URL:     remotePricingURL,
			Timeout: 15 * time.Second,
		},
		logger:   logger,
		interval: 6 * time.Hour,
		stopCh:   make(chan struct{}),
	}
}

// Start begins background polling. Non-blocking.
func (u *PricingUpdater) Start() {
	// Fetch once immediately (best-effort, don't block startup)
	go u.fetchAndApply()

	go func() {
		ticker := time.NewTicker(u.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				u.fetchAndApply()
			case <-u.stopCh:
				return
			}
		}
	}()
}

// Stop stops the background polling.
func (u *PricingUpdater) Stop() {
	close(u.stopCh)
}

// SetMediaPricingApplier sets the callback for applying media model pricing.
func (u *PricingUpdater) SetMediaPricingApplier(applier MediaPricingApplier) {
	var cachedContent string
	u.mu.Lock()
	u.mediaPricingApplier = applier
	cachedContent = u.cachedContent
	u.mu.Unlock()
	if applier != nil && cachedContent != "" {
		u.applyMediaPricing(cachedContent, applier)
	}
}

func (u *PricingUpdater) fetchAndApply() {
	u.fetchMu.Lock()
	content, etag, err := u.dl.FetchIfChanged()
	u.fetchMu.Unlock()

	if err != nil {
		if u.logger != nil {
			u.logger.Debug("pricing update fetch failed (will retry)", zap.Error(err))
		}
		return
	}
	if content == "" {
		return
	}
	// ETag unchanged means FetchIfChanged returned cached content — skip re-apply
	u.mu.RLock()
	lastETag := u.lastETag
	applier := u.mediaPricingApplier
	u.mu.RUnlock()
	if etag != "" && etag == lastETag {
		return
	}

	var data remotePricingJSON
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		if u.logger != nil {
			u.logger.Warn("pricing update: invalid JSON", zap.Error(err))
		}
		return
	}

	applied := 0
	for modelID, item := range data.Models {
		BuiltinModelPricing[modelID] = &ModelPricing{
			ModelID:     modelID,
			InputPrice:  item.Input,
			OutputPrice: item.Output,
			CachePrice:  item.Cache,
		}
		applied++
	}

	// Apply media model pricing via callback
	mediaApplied := u.applyMediaPricingItems(data.MediaModels, applier)

	u.mu.Lock()
	u.cachedContent = content
	u.lastETag = etag
	u.mu.Unlock()
	if u.logger != nil {
		u.logger.Info("remote pricing applied",
			zap.String("etag", etag),
			zap.Int("models", applied),
			zap.Int("media_models", mediaApplied))
	}
}

func (u *PricingUpdater) applyMediaPricing(content string, applier MediaPricingApplier) int {
	if applier == nil || content == "" {
		return 0
	}
	var data remotePricingJSON
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		if u.logger != nil {
			u.logger.Warn("pricing update: invalid cached JSON", zap.Error(err))
		}
		return 0
	}
	return u.applyMediaPricingItems(data.MediaModels, applier)
}

func (u *PricingUpdater) applyMediaPricingItems(items map[string]remoteMediaPricingItem, applier MediaPricingApplier) int {
	if applier == nil || len(items) == 0 {
		return 0
	}
	applied := 0
	for modelID, item := range items {
		if item.Unit == "" || modelID == "_comment" {
			continue
		}
		applier(modelID, item.Output, item.Unit)
		applied++
	}
	return applied
}
