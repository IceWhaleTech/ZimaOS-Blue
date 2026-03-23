package providerpool

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
)

func TestPricingUpdater_SetMediaPricingApplierDoesNotBlockDuringFetch(t *testing.T) {
	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	mediaApplied := make(chan struct{}, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(fetchStarted)
		<-releaseFetch
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"media_models":{"demo-model":{"output":1.25,"unit":"image"}}}`))
	}))
	defer srv.Close()

	updater := &PricingUpdater{
		dl: &downloader.Downloader{
			URL:     srv.URL,
			Timeout: 2 * time.Second,
		},
		interval: 6 * time.Hour,
		stopCh:   make(chan struct{}),
	}

	fetchDone := make(chan struct{})
	go func() {
		updater.fetchAndApply()
		close(fetchDone)
	}()

	select {
	case <-fetchStarted:
	case <-time.After(time.Second):
		t.Fatal("fetch did not start")
	}

	setDone := make(chan struct{})
	go func() {
		updater.SetMediaPricingApplier(func(modelID string, outputPrice float64, unit string) {
			if modelID == "demo-model" && outputPrice == 1.25 && unit == "image" {
				select {
				case mediaApplied <- struct{}{}:
				default:
				}
			}
		})
		close(setDone)
	}()

	select {
	case <-setDone:
	case <-time.After(150 * time.Millisecond):
		t.Fatal("SetMediaPricingApplier blocked on in-flight fetch")
	}

	close(releaseFetch)

	select {
	case <-fetchDone:
	case <-time.After(time.Second):
		t.Fatal("fetch did not complete")
	}

	select {
	case <-mediaApplied:
	case <-time.After(time.Second):
		t.Fatal("expected media pricing applier to run after fetch completed")
	}
}

func TestPricingUpdater_SetMediaPricingApplierReplaysCachedContent(t *testing.T) {
	updater := &PricingUpdater{
		cachedContent: `{"media_models":{"cached-model":{"output":2.5,"unit":"video"}}}`,
	}

	mediaApplied := make(chan struct{}, 1)
	updater.SetMediaPricingApplier(func(modelID string, outputPrice float64, unit string) {
		if modelID == "cached-model" && outputPrice == 2.5 && unit == "video" {
			mediaApplied <- struct{}{}
		}
	})

	select {
	case <-mediaApplied:
	case <-time.After(time.Second):
		t.Fatal("expected cached media pricing to be applied immediately")
	}
}
