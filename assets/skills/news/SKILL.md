---
name: news
description: Retrieve latest news headlines and article search results via NewsAPI.
---

# News

Get latest news headlines and articles.

## API: NewsAPI

- Endpoint: `https://newsapi.org/v2`
- Sign up: https://newsapi.org/register (free tier: 100 requests/day)

## Usage

```bash
# Top headlines by country
curl -s "https://newsapi.org/v2/top-headlines?country=us&pageSize=5&apiKey=$NEWS_API_KEY" | jq '.articles[] | {title, source: .source.name, url, publishedAt}'

# Top headlines by category
curl -s "https://newsapi.org/v2/top-headlines?country=us&category=technology&pageSize=5&apiKey=$NEWS_API_KEY" | jq '.articles[] | {title, source: .source.name, url}'

# Search news
curl -s "https://newsapi.org/v2/everything?q=AI&sortBy=publishedAt&pageSize=5&apiKey=$NEWS_API_KEY" | jq '.articles[] | {title, source: .source.name, url, publishedAt}'
```

## Setup

1. Get API key from https://newsapi.org/register
2. Set environment variable: `export NEWS_API_KEY=your_key`

## Parameters

- `query`: Search keywords (for everything endpoint)
- `category`: business, entertainment, general, health, science, sports, technology
- `country`: 2-letter ISO country code (us, cn, gb, jp, etc.)
- `count`: Number of articles (default: 5, max: 100)

## Notes

- Free tier: 100 requests/day, articles up to 1 month old
- Categories only work with top-headlines endpoint
- Use `everything` endpoint for keyword search across all sources
