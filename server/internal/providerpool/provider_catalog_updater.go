package providerpool

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
	"go.uber.org/zap"
)

const remoteOfficialProviderCatalogURL = "https://raw.githubusercontent.com/IceWhaleTech/ZimaOS-Blue/main/docs/provider_catalog.json"

type ProviderCatalogStatus struct {
	ETag          string    `json:"etag,omitempty"`
	LastUpdatedAt time.Time `json:"last_updated_at,omitempty"`
	SourceURL     string    `json:"source_url"`
	FallbackInUse bool      `json:"fallback_in_use"`
}

type ProviderCatalogUpdater struct {
	dl       *downloader.Downloader
	logger   *zap.Logger
	interval time.Duration
	stopCh   chan struct{}
	fetchMu  sync.Mutex

	mu            sync.RWMutex
	lastETag      string
	lastUpdatedAt time.Time
	fallbackInUse bool
	onApplied     func()
}

func NewProviderCatalogUpdater(logger *zap.Logger) *ProviderCatalogUpdater {
	return &ProviderCatalogUpdater{
		dl: &downloader.Downloader{
			URL:     remoteOfficialProviderCatalogURL,
			Timeout: 15 * time.Second,
		},
		logger:        logger,
		interval:      6 * time.Hour,
		stopCh:        make(chan struct{}),
		fallbackInUse: true,
	}
}

func (u *ProviderCatalogUpdater) SetApplyCallback(callback func()) {
	u.mu.Lock()
	u.onApplied = callback
	u.mu.Unlock()
}

func (u *ProviderCatalogUpdater) Start() {
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

func (u *ProviderCatalogUpdater) Stop() {
	close(u.stopCh)
}

func (u *ProviderCatalogUpdater) Status() ProviderCatalogStatus {
	u.mu.RLock()
	defer u.mu.RUnlock()

	return ProviderCatalogStatus{
		ETag:          u.lastETag,
		LastUpdatedAt: u.lastUpdatedAt,
		SourceURL:     remoteOfficialProviderCatalogURL,
		FallbackInUse: u.fallbackInUse,
	}
}

func (u *ProviderCatalogUpdater) fetchAndApply() {
	u.fetchMu.Lock()
	content, etag, err := u.dl.FetchIfChanged()
	u.fetchMu.Unlock()

	if err != nil {
		if u.logger != nil {
			u.logger.Debug("official provider catalog fetch failed (will retry)", zap.Error(err))
		}
		return
	}
	if content == "" {
		return
	}

	u.mu.RLock()
	lastETag := u.lastETag
	callback := u.onApplied
	u.mu.RUnlock()
	if etag != "" && etag == lastETag {
		return
	}

	var catalog officialProviderCatalog
	if err := json.Unmarshal([]byte(content), &catalog); err != nil {
		if u.logger != nil {
			u.logger.Warn("official provider catalog: invalid JSON", zap.Error(err))
		}
		return
	}

	SetOfficialProviderCatalog(catalog)

	now := time.Now()
	u.mu.Lock()
	u.lastETag = etag
	u.lastUpdatedAt = now
	u.fallbackInUse = false
	u.mu.Unlock()

	if callback != nil {
		callback()
	}

	if u.logger != nil {
		u.logger.Info("official provider catalog applied",
			zap.String("etag", etag),
			zap.Int("providers", len(catalog.Providers)),
			zap.Int("provider_models", len(catalog.Models)))
	}
}
