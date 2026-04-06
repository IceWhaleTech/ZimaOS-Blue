package providerpool

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
)

func resetBuiltinPricingDataForTest(t *testing.T) {
	t.Helper()

	originalBuiltinModelPricing := BuiltinModelPricing
	originalBuiltinModelFamilies := builtinModelFamilies

	BuiltinModelPricing = nil
	builtinModelFamilies = nil
	builtinPricingDataOnce = sync.Once{}

	t.Cleanup(func() {
		BuiltinModelPricing = originalBuiltinModelPricing
		builtinModelFamilies = originalBuiltinModelFamilies
		builtinPricingDataOnce = sync.Once{}
		if originalBuiltinModelPricing != nil || originalBuiltinModelFamilies != nil {
			builtinPricingDataOnce.Do(func() {})
		}
	})
}

func TestMatchModelPricing_InitializesBuiltinPricingDataOnDemand(t *testing.T) {
	resetBuiltinPricingDataForTest(t)

	if BuiltinModelPricing != nil || builtinModelFamilies != nil {
		t.Fatal("expected builtin pricing data to start uninitialized")
	}

	pricing := MatchModelPricing("gpt-4o")
	if pricing == nil {
		t.Fatal("MatchModelPricing() = nil, want lazy initialization to restore builtin pricing")
	}
	if pricing.InputPrice != 2.5 {
		t.Fatalf("MatchModelPricing().InputPrice = %v, want %v", pricing.InputPrice, 2.5)
	}
	if len(BuiltinModelPricing) == 0 || len(builtinModelFamilies) == 0 {
		t.Fatal("expected builtin pricing data to be initialized after first pricing lookup")
	}
}

func TestPricingUpdater_FetchAndApply_InitializesBuiltinPricingDataOnDemand(t *testing.T) {
	resetBuiltinPricingDataForTest(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":{"demo-remote":{"input":0.42,"output":0.84,"cache":0.21}}}`))
	}))
	defer srv.Close()

	updater := &PricingUpdater{
		dl: &downloader.Downloader{
			URL:     srv.URL,
			Timeout: time.Second,
		},
		interval: time.Hour,
		stopCh:   make(chan struct{}),
	}

	updater.fetchAndApply()

	pricing := BuiltinModelPricing["demo-remote"]
	if pricing == nil {
		t.Fatal("expected fetchAndApply() to initialize builtin pricing data before applying remote pricing")
	}
	if pricing.InputPrice != 0.42 || pricing.OutputPrice != 0.84 || pricing.CachePrice != 0.21 {
		t.Fatalf("remote pricing = %+v, want input=%v output=%v cache=%v", pricing, 0.42, 0.84, 0.21)
	}
}
