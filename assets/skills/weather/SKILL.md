---
name: weather
description: Get current weather and forecast data for locations via OpenWeatherMap APIs.
---

# Weather

Get current weather and forecast for any location.

## API: OpenWeatherMap

- Endpoint: `https://api.openweathermap.org/data/2.5/weather`
- Sign up: https://openweathermap.org/api (free tier: 1000 calls/day)

## Usage

```bash
# Current weather (add &lang=LANG_CODE to localize description)
curl -s "https://api.openweathermap.org/data/2.5/weather?q=Beijing&units=metric&lang=zh_cn&appid=$OPENWEATHER_API_KEY" | jq '{temp: .main.temp, feels_like: .main.feels_like, humidity: .main.humidity, description: .weather[0].description, wind: .wind.speed}'

# 5-day forecast
curl -s "https://api.openweathermap.org/data/2.5/forecast?q=Beijing&units=metric&cnt=5&lang=zh_cn&appid=$OPENWEATHER_API_KEY" | jq '.list[] | {dt_txt, temp: .main.temp, description: .weather[0].description}'
```

## Setup

1. Get API key from https://openweathermap.org/api
2. Set environment variable: `export OPENWEATHER_API_KEY=your_key`

## Parameters

- `location`: City name (e.g., "Beijing", "New York,US", "London,GB")
- `units`: "metric" (Celsius) or "imperial" (Fahrenheit), default: metric

## Localization

Set the `lang` parameter based on the user's locale to get localized weather descriptions.

Locale mapping (BCP-47 → OpenWeatherMap `lang`):
- `zh-CN` → `zh_cn`, `zh-TW` → `zh_tw`
- `ja-JP` → `ja`, `ko-KR` → `kr`
- `en-US`/`en-GB` → `en`
- `fr-FR` → `fr`, `de-DE` → `de`, `es-ES` → `es`, `it-IT` → `it`
- `pt-BR` → `pt_br`, `ru-RU` → `ru`, `pl-PL` → `pl`, `nl-NL` → `nl`
- `sv-SE` → `se`, `da-DK` → `da`, `nb-NO` → `no`
- Full list: https://openweathermap.org/current#multi

Always respond in the user's language.

## Notes

- Free tier supports current weather + 5-day forecast
- Use `q=CityName,CountryCode` for disambiguation
- Response includes: temperature, humidity, wind speed, weather description
