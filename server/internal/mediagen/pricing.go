package mediagen

import "sync"

// MediaPricingUnit indicates how a media model is priced.
type MediaPricingUnit string

const (
	PricingPerImage  MediaPricingUnit = "image"  // USD per image
	PricingPerSecond MediaPricingUnit = "second" // USD per second of video
	PricingPerVideo  MediaPricingUnit = "video"  // USD per video (flat rate)
)

// MediaModelPricing holds pricing for a single media model.
type MediaModelPricing struct {
	OutputPrice float64          `json:"output"` // price per unit
	Unit        MediaPricingUnit `json:"unit"`
}

// CalculateCost returns the estimated cost for a media generation.
//   - For "image": cost = OutputPrice * imageCount
//   - For "second": cost = OutputPrice * durationSec
//   - For "video": cost = OutputPrice (flat)
func (p *MediaModelPricing) CalculateCost(imageCount int, durationSec float64) float64 {
	switch p.Unit {
	case PricingPerImage:
		if imageCount <= 0 {
			imageCount = 1
		}
		return p.OutputPrice * float64(imageCount)
	case PricingPerSecond:
		return p.OutputPrice * durationSec
	case PricingPerVideo:
		return p.OutputPrice
	default:
		return 0
	}
}

// builtinMediaPricing contains known media model pricing.
// Updated from docs/model_pricing.json media_models section.
var (
	builtinMediaPricing map[string]*MediaModelPricing
	mediaPricingOnce    sync.Once
)

func ensureMediaPricing() {
	mediaPricingOnce.Do(func() {
		builtinMediaPricing = buildBuiltinMediaPricing()
	})
}

func buildBuiltinMediaPricing() map[string]*MediaModelPricing {
	return map[string]*MediaModelPricing{
		// --- Image models (per image) ---
		"nano-banana-pro":                       {OutputPrice: 0.15, Unit: PricingPerImage},
		"nano-banana":                           {OutputPrice: 0.039, Unit: PricingPerImage},
		"gemini-2.0-flash-exp-image-generation": {OutputPrice: 0.039, Unit: PricingPerImage},
		"imagen-3.0-generate-002":               {OutputPrice: 0.04, Unit: PricingPerImage},
		"imagen-4.0-generate-001":               {OutputPrice: 0.04, Unit: PricingPerImage},
		"imagen-4.0-fast-generate-001":          {OutputPrice: 0.02, Unit: PricingPerImage},
		"imagen-4.0-ultra-generate-001":         {OutputPrice: 0.06, Unit: PricingPerImage},
		"midjourney":                            {OutputPrice: 0.085, Unit: PricingPerImage},
		"qwen-image-max":                        {OutputPrice: 0.03, Unit: PricingPerImage},
		"qwen-image-edit-max":                   {OutputPrice: 0.03, Unit: PricingPerImage},
		"wan2.6-t2i":                            {OutputPrice: 0.03, Unit: PricingPerImage},
		"wan2.6-image":                          {OutputPrice: 0.03, Unit: PricingPerImage},
		"wan2.5-t2i-preview":                    {OutputPrice: 0.03, Unit: PricingPerImage},
		"wan2.5-i2i-preview":                    {OutputPrice: 0.03, Unit: PricingPerImage},
		"wanx-v1":                               {OutputPrice: 0.008, Unit: PricingPerImage},
		"wanx2.1-t2i-turbo":                     {OutputPrice: 0.008, Unit: PricingPerImage},

		// --- Video models (per second) ---
		"wan2.6-t2v":         {OutputPrice: 0.10, Unit: PricingPerSecond},
		"wan2.6-i2v":         {OutputPrice: 0.10, Unit: PricingPerSecond},
		"wan2.5-t2v-preview": {OutputPrice: 0.05, Unit: PricingPerSecond},
		"wan2.5-i2v-preview": {OutputPrice: 0.05, Unit: PricingPerSecond},
		"wan2.5-t2v-spark":   {OutputPrice: 0.05, Unit: PricingPerSecond},
		"wan2.6-t2v-spark":   {OutputPrice: 0.05, Unit: PricingPerSecond},
		"wan2.5-i2v-spark":   {OutputPrice: 0.05, Unit: PricingPerSecond},
		"wan2.6-i2v-spark":   {OutputPrice: 0.05, Unit: PricingPerSecond},
		"wan2.2-t2v-plus":    {OutputPrice: 0.02, Unit: PricingPerSecond},
		"wan2.2-i2v-plus":    {OutputPrice: 0.02, Unit: PricingPerSecond},
		"wan2.2-i2v-flash":   {OutputPrice: 0.015, Unit: PricingPerSecond},
		"wan2.1-kf2v-plus":   {OutputPrice: 0.10, Unit: PricingPerSecond},
		"wan2.1-vace-plus":   {OutputPrice: 0.10, Unit: PricingPerSecond},
		"wan2-spark-t2v":     {OutputPrice: 0.05, Unit: PricingPerSecond},

		// --- Video models (flat per video) ---
		"midjourney-video": {OutputPrice: 0.51, Unit: PricingPerVideo},

		// --- MiniMax Hailuo video models (flat per video) ---
		"MiniMax-Hailuo-2.3":      {OutputPrice: 0.30, Unit: PricingPerVideo},
		"MiniMax-Hailuo-2.3-Fast": {OutputPrice: 0.15, Unit: PricingPerVideo},
		"MiniMax-Hailuo-02":       {OutputPrice: 0.25, Unit: PricingPerVideo},
		"T2V-01":                  {OutputPrice: 0.20, Unit: PricingPerVideo},
		"T2V-01-Director":         {OutputPrice: 0.25, Unit: PricingPerVideo},
		"I2V-01":                  {OutputPrice: 0.20, Unit: PricingPerVideo},
		"I2V-01-Director":         {OutputPrice: 0.25, Unit: PricingPerVideo},
		"I2V-01-live":             {OutputPrice: 0.15, Unit: PricingPerVideo},
		"S2V-01":                  {OutputPrice: 0.25, Unit: PricingPerVideo},
	}
}

// mediaPricingMu protects runtime updates to the pricing map.
var mediaPricingMu sync.RWMutex

// GetMediaModelPricing returns pricing for a media model, or nil if unknown.
func GetMediaModelPricing(modelID string) *MediaModelPricing {
	ensureMediaPricing()
	mediaPricingMu.RLock()
	defer mediaPricingMu.RUnlock()
	return builtinMediaPricing[modelID]
}

// SetMediaModelPricing sets or updates pricing for a media model (used by PricingUpdater).
func SetMediaModelPricing(modelID string, pricing *MediaModelPricing) {
	ensureMediaPricing()
	mediaPricingMu.Lock()
	defer mediaPricingMu.Unlock()
	builtinMediaPricing[modelID] = pricing
}

// EnrichModelPricing fills in Price and PricingUnit on each model from the builtin pricing map.
func EnrichModelPricing(models []MediaModelInfo) {
	ensureMediaPricing()
	mediaPricingMu.RLock()
	defer mediaPricingMu.RUnlock()
	for i := range models {
		if p := builtinMediaPricing[models[i].ID]; p != nil {
			models[i].Price = p.OutputPrice
			models[i].PricingUnit = string(p.Unit)
		}
	}
}

// CalculateMediaCost calculates the cost for a completed media task.
func CalculateMediaCost(modelID string, imageCount int, durationSec float64) float64 {
	pricing := GetMediaModelPricing(modelID)
	if pricing == nil {
		return 0
	}
	return pricing.CalculateCost(imageCount, durationSec)
}
