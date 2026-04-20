package mediagen

import (
	"testing"
)

func TestMediaModelPricing_CalculateCost_PerImage(t *testing.T) {
	p := &MediaModelPricing{OutputPrice: 0.04, Unit: PricingPerImage}

	tests := []struct {
		name       string
		imageCount int
		want       float64
	}{
		{"single image", 1, 0.04},
		{"three images", 3, 0.12},
		{"zero defaults to 1", 0, 0.04},
		{"negative defaults to 1", -1, 0.04},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.CalculateCost(tt.imageCount, 0)
			if got != tt.want {
				t.Errorf("CalculateCost(%d, 0) = %f, want %f", tt.imageCount, got, tt.want)
			}
		})
	}
}

func TestMediaModelPricing_CalculateCost_PerSecond(t *testing.T) {
	p := &MediaModelPricing{OutputPrice: 0.10, Unit: PricingPerSecond}

	tests := []struct {
		name        string
		durationSec float64
		want        float64
	}{
		{"5 seconds", 5.0, 0.50},
		{"10 seconds", 10.0, 1.00},
		{"zero duration", 0, 0},
		{"fractional", 2.5, 0.25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.CalculateCost(0, tt.durationSec)
			if got != tt.want {
				t.Errorf("CalculateCost(0, %f) = %f, want %f", tt.durationSec, got, tt.want)
			}
		})
	}
}

func TestMediaModelPricing_CalculateCost_PerVideo(t *testing.T) {
	p := &MediaModelPricing{OutputPrice: 0.51, Unit: PricingPerVideo}

	// Flat rate regardless of count or duration
	got := p.CalculateCost(1, 10.0)
	if got != 0.51 {
		t.Errorf("CalculateCost(1, 10) = %f, want 0.51", got)
	}
}

func TestMediaModelPricing_CalculateCost_UnknownUnit(t *testing.T) {
	p := &MediaModelPricing{OutputPrice: 1.0, Unit: "unknown"}
	got := p.CalculateCost(1, 10.0)
	if got != 0 {
		t.Errorf("CalculateCost with unknown unit = %f, want 0", got)
	}
}

func TestGetMediaModelPricing_Builtin(t *testing.T) {
	// Test known builtin models
	tests := []struct {
		modelID  string
		wantUnit MediaPricingUnit
	}{
		{"nano-banana-pro", PricingPerImage},
		{"qwen-image-max", PricingPerImage},
		{"wan2.6-t2v", PricingPerSecond},
		{"midjourney-video", PricingPerVideo},
	}

	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			p := GetMediaModelPricing(tt.modelID)
			if p == nil {
				t.Fatalf("GetMediaModelPricing(%q) = nil", tt.modelID)
			}
			if p.Unit != tt.wantUnit {
				t.Errorf("Unit = %q, want %q", p.Unit, tt.wantUnit)
			}
			if p.OutputPrice <= 0 {
				t.Errorf("OutputPrice = %f, want > 0", p.OutputPrice)
			}
		})
	}
}

func TestGetMediaModelPricing_Unknown(t *testing.T) {
	p := GetMediaModelPricing("nonexistent-model-xyz")
	if p != nil {
		t.Errorf("GetMediaModelPricing(unknown) = %v, want nil", p)
	}
}

func TestSetMediaModelPricing(t *testing.T) {
	modelID := "test-model-pricing-set"

	// Should not exist initially
	if p := GetMediaModelPricing(modelID); p != nil {
		t.Fatalf("model should not exist before set")
	}

	// Set it
	SetMediaModelPricing(modelID, &MediaModelPricing{OutputPrice: 0.99, Unit: PricingPerImage})

	// Should exist now
	p := GetMediaModelPricing(modelID)
	if p == nil {
		t.Fatal("GetMediaModelPricing returned nil after Set")
	}
	if p.OutputPrice != 0.99 {
		t.Errorf("OutputPrice = %f, want 0.99", p.OutputPrice)
	}

	// Clean up
	mediaPricingMu.Lock()
	delete(builtinMediaPricing, modelID)
	mediaPricingMu.Unlock()
}

func TestCalculateMediaCost(t *testing.T) {
	tests := []struct {
		name        string
		modelID     string
		imageCount  int
		durationSec float64
		want        float64
	}{
		{"image model", "qwen-image-max", 2, 0, 0.06},
		{"video model per second", "wan2.6-t2v", 0, 5.0, 0.50},
		{"video model flat", "midjourney-video", 0, 0, 0.51},
		{"unknown model", "nonexistent", 1, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateMediaCost(tt.modelID, tt.imageCount, tt.durationSec)
			if got != tt.want {
				t.Errorf("CalculateMediaCost(%q, %d, %f) = %f, want %f",
					tt.modelID, tt.imageCount, tt.durationSec, got, tt.want)
			}
		})
	}
}
