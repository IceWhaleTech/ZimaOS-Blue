package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// Weather is a built-in weather skill
type Weather struct {
	manifest   *skill.Manifest
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

// WeatherConfig contains configuration for the weather skill
type WeatherConfig struct {
	APIKey  string
	BaseURL string
}

// NewWeather creates a new weather skill
func NewWeather(config *WeatherConfig) *Weather {
	baseURL := "https://api.openweathermap.org/data/2.5"
	apiKey := ""

	if config != nil {
		if config.BaseURL != "" {
			baseURL = config.BaseURL
		}
		if config.APIKey != "" {
			apiKey = config.APIKey
		}
	}

	return &Weather{
		manifest: &skill.Manifest{
			ID:          "weather",
			Name:        "Weather",
			Version:     "1.0.0",
			Description: "Get current weather information for a location",
			Category:    "information",
			Icon:        "weather",
			Tags:        []string{"weather", "forecast", "utility"},
			Inputs: []skill.Parameter{
				{
					Name:        "location",
					Type:        "string",
					Description: "City name or location (e.g., 'London', 'New York,US')",
					Required:    true,
				},
				{
					Name:        "units",
					Type:        "string",
					Description: "Temperature units: 'metric' (Celsius), 'imperial' (Fahrenheit), 'standard' (Kelvin)",
					Required:    false,
					Default:     "metric",
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "weather",
					Type:        "object",
					Description: "Weather information including temperature, humidity, conditions",
				},
			},
			Permissions: []string{"network.http"},
			Config: map[string]any{
				"api_key_required": true,
				"rate_limit":       60, // requests per minute
			},
		},
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		apiKey:  apiKey,
		baseURL: baseURL,
	}
}

// Manifest returns the skill manifest
func (w *Weather) Manifest() *skill.Manifest {
	return w.manifest
}

// Validate validates the input parameters
func (w *Weather) Validate(input map[string]any) error {
	location, ok := input["location"]
	if !ok {
		return fmt.Errorf("location is required")
	}

	if _, ok := location.(string); !ok {
		return fmt.Errorf("location must be a string")
	}

	if units, ok := input["units"]; ok {
		unitsStr, ok := units.(string)
		if !ok {
			return fmt.Errorf("units must be a string")
		}
		validUnits := map[string]bool{"metric": true, "imperial": true, "standard": true}
		if !validUnits[unitsStr] {
			return fmt.Errorf("invalid units: %s (must be metric, imperial, or standard)", unitsStr)
		}
	}

	return nil
}

// Execute executes the weather skill
func (w *Weather) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	location := input["location"].(string)
	units := "metric"
	if u, ok := input["units"].(string); ok {
		units = u
	}

	// Check if API key is configured
	if w.apiKey == "" {
		// Return mock data when no API key is configured
		return w.getMockWeather(location, units), nil
	}

	// Build request URL
	params := url.Values{}
	params.Set("q", location)
	params.Set("units", units)
	params.Set("appid", w.apiKey)

	reqURL := fmt.Sprintf("%s/weather?%s", w.baseURL, params.Encode())

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return skill.NewErrorResult(fmt.Errorf("failed to create request: %w", err)), nil
	}

	// Execute request
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return skill.NewErrorResult(fmt.Errorf("failed to fetch weather: %w", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return skill.NewErrorResult(fmt.Errorf("weather API returned status %d", resp.StatusCode)), nil
	}

	// Parse response
	var weatherResp openWeatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&weatherResp); err != nil {
		return skill.NewErrorResult(fmt.Errorf("failed to parse weather response: %w", err)), nil
	}

	// Build result
	result := w.buildWeatherResult(&weatherResp, units)
	return skill.NewResult(result), nil
}

// getMockWeather returns mock weather data for testing
func (w *Weather) getMockWeather(location, units string) *skill.Result {
	tempUnit := "°C"
	temp := 20.0
	feelsLike := 18.0

	if units == "imperial" {
		tempUnit = "°F"
		temp = 68.0
		feelsLike = 64.4
	} else if units == "standard" {
		tempUnit = "K"
		temp = 293.15
		feelsLike = 291.15
	}

	return skill.NewResult(map[string]any{
		"location": map[string]any{
			"name":    location,
			"country": "Unknown",
		},
		"temperature": map[string]any{
			"current":    temp,
			"feels_like": feelsLike,
			"min":        temp - 2,
			"max":        temp + 3,
			"unit":       tempUnit,
		},
		"conditions": map[string]any{
			"main":        "Clear",
			"description": "clear sky",
			"icon":        "01d",
		},
		"humidity":   65,
		"pressure":   1013,
		"visibility": 10000,
		"wind": map[string]any{
			"speed":     3.5,
			"direction": 180,
		},
		"clouds":    10,
		"timestamp": time.Now().Unix(),
		"mock":      true,
		"note":      "This is mock data. Configure API key for real weather data.",
	})
}

// buildWeatherResult builds the weather result from API response
func (w *Weather) buildWeatherResult(resp *openWeatherResponse, units string) map[string]any {
	tempUnit := "°C"
	if units == "imperial" {
		tempUnit = "°F"
	} else if units == "standard" {
		tempUnit = "K"
	}

	result := map[string]any{
		"location": map[string]any{
			"name":    resp.Name,
			"country": resp.Sys.Country,
			"lat":     resp.Coord.Lat,
			"lon":     resp.Coord.Lon,
		},
		"temperature": map[string]any{
			"current":    resp.Main.Temp,
			"feels_like": resp.Main.FeelsLike,
			"min":        resp.Main.TempMin,
			"max":        resp.Main.TempMax,
			"unit":       tempUnit,
		},
		"humidity":   resp.Main.Humidity,
		"pressure":   resp.Main.Pressure,
		"visibility": resp.Visibility,
		"wind": map[string]any{
			"speed":     resp.Wind.Speed,
			"direction": resp.Wind.Deg,
		},
		"clouds":    resp.Clouds.All,
		"timestamp": resp.Dt,
	}

	if len(resp.Weather) > 0 {
		result["conditions"] = map[string]any{
			"main":        resp.Weather[0].Main,
			"description": resp.Weather[0].Description,
			"icon":        resp.Weather[0].Icon,
		}
	}

	return result
}

// openWeatherResponse represents the OpenWeatherMap API response
type openWeatherResponse struct {
	Coord struct {
		Lon float64 `json:"lon"`
		Lat float64 `json:"lat"`
	} `json:"coord"`
	Weather []struct {
		ID          int    `json:"id"`
		Main        string `json:"main"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	} `json:"weather"`
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		TempMin   float64 `json:"temp_min"`
		TempMax   float64 `json:"temp_max"`
		Pressure  int     `json:"pressure"`
		Humidity  int     `json:"humidity"`
	} `json:"main"`
	Visibility int `json:"visibility"`
	Wind       struct {
		Speed float64 `json:"speed"`
		Deg   int     `json:"deg"`
	} `json:"wind"`
	Clouds struct {
		All int `json:"all"`
	} `json:"clouds"`
	Dt   int64  `json:"dt"`
	Name string `json:"name"`
	Sys  struct {
		Country string `json:"country"`
	} `json:"sys"`
}
