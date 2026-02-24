# Weather

Get current weather and forecast for any location.

## API: OpenWeatherMap

- Endpoint: `https://api.openweathermap.org/data/2.5/weather`
- Sign up: https://openweathermap.org/api (free tier: 1000 calls/day)

## Usage

```bash
# Current weather
curl -s "https://api.openweathermap.org/data/2.5/weather?q=Beijing&units=metric&appid=$OPENWEATHER_API_KEY" | jq '{temp: .main.temp, feels_like: .main.feels_like, humidity: .main.humidity, description: .weather[0].description, wind: .wind.speed}'

# 5-day forecast
curl -s "https://api.openweathermap.org/data/2.5/forecast?q=Beijing&units=metric&cnt=5&appid=$OPENWEATHER_API_KEY" | jq '.list[] | {dt_txt, temp: .main.temp, description: .weather[0].description}'
```

## Setup

1. Get API key from https://openweathermap.org/api
2. Set environment variable: `export OPENWEATHER_API_KEY=your_key`

## Parameters

- `location`: City name (e.g., "Beijing", "New York,US", "London,GB")
- `units`: "metric" (Celsius) or "imperial" (Fahrenheit), default: metric

## Notes

- Free tier supports current weather + 5-day forecast
- Use `q=CityName,CountryCode` for disambiguation
- Response includes: temperature, humidity, wind speed, weather description
