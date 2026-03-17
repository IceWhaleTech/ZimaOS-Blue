package skillmarket

import (
	"math"
	"time"
)

func computePopularityScore(stars, downloads int) float64 {
	starsNorm := math.Min(1, math.Log1p(float64(stars))/math.Log1p(5000))
	downloadsNorm := math.Min(1, math.Log1p(float64(downloads))/math.Log1p(100000))
	return 100 * (0.45*starsNorm + 0.55*downloadsNorm)
}

func computeTrendingScore(stars, downloads int, updatedAt time.Time, securityScore int) float64 {
	starsNorm := math.Min(1, math.Log1p(float64(stars))/math.Log1p(5000))
	downloadsNorm := math.Min(1, math.Log1p(float64(downloads))/math.Log1p(100000))
	ageDays := time.Since(updatedAt).Hours() / 24
	recentNorm := math.Max(0, 1-(ageDays/90))
	securityNorm := float64(securityScore) / 100
	return 100 * (0.4*starsNorm + 0.3*downloadsNorm + 0.2*recentNorm + 0.1*securityNorm)
}
