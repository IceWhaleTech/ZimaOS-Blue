---
name: stocks
description: Retrieve stock quotes and market data via Alpha Vantage APIs.
---

# Stocks

Get stock market data and quotes.

## API: Alpha Vantage

- Endpoint: `https://www.alphavantage.co/query`
- Sign up: https://www.alphavantage.co/support/#api-key (free tier: 25 requests/day)

## Usage

```bash
# Real-time quote
curl -s "https://www.alphavantage.co/query?function=GLOBAL_QUOTE&symbol=AAPL&apikey=$ALPHA_VANTAGE_KEY" | jq '."Global Quote" | {symbol: ."01. symbol", price: ."05. price", change: ."09. change", change_pct: ."10. change percent", volume: ."06. volume"}'

# Daily time series (last 5 days)
curl -s "https://www.alphavantage.co/query?function=TIME_SERIES_DAILY&symbol=AAPL&outputsize=compact&apikey=$ALPHA_VANTAGE_KEY" | jq '."Time Series (Daily)" | to_entries[:5][] | {date: .key, open: .value."1. open", high: .value."2. high", low: .value."3. low", close: .value."4. close"}'

# Symbol search
curl -s "https://www.alphavantage.co/query?function=SYMBOL_SEARCH&keywords=apple&apikey=$ALPHA_VANTAGE_KEY" | jq '.bestMatches[] | {symbol: ."1. symbol", name: ."2. name", region: ."4. region"}'
```

## Setup

1. Get API key from https://www.alphavantage.co/support/#api-key
2. Set environment variable: `export ALPHA_VANTAGE_KEY=your_key`

## Parameters

- `symbol`: Stock ticker symbol (e.g., "AAPL", "GOOGL", "MSFT")
- `function`: GLOBAL_QUOTE (real-time), TIME_SERIES_DAILY, SYMBOL_SEARCH

## Notes

- Free tier: 25 requests/day, 5 requests/minute
- Use SYMBOL_SEARCH to find ticker symbols
- Supports US and international markets
