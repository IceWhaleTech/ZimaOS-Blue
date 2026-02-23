# Crypto

Get cryptocurrency prices and market data.

## API: CoinGecko

- Endpoint: `https://api.coingecko.com/api/v3`
- Docs: https://docs.coingecko.com/v3.0.1/reference/introduction
- Free tier: 30 calls/min (no API key required for basic usage)

## Usage

```bash
# Current price (no API key needed)
curl -s "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin,ethereum&vs_currencies=usd,cny&include_24hr_change=true" | jq .

# Top coins by market cap
curl -s "https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&order=market_cap_desc&per_page=10&page=1" | jq '.[] | {name, symbol, current_price, price_change_percentage_24h, market_cap}'

# Coin details
curl -s "https://api.coingecko.com/api/v3/coins/bitcoin" | jq '{name, symbol, price_usd: .market_data.current_price.usd, ath: .market_data.ath.usd, market_cap: .market_data.market_cap.usd}'

# Search coins
curl -s "https://api.coingecko.com/api/v3/search?query=solana" | jq '.coins[:5][] | {id, name, symbol, market_cap_rank}'
```

## Setup (Optional)

CoinGecko free tier works without API key (30 calls/min). For higher limits:

1. Get API key from https://www.coingecko.com/en/api/pricing
2. Set environment variable: `export COINGECKO_API_KEY=your_key`
3. Add header: `-H "x-cg-demo-api-key: $COINGECKO_API_KEY"`

## Parameters

- `coin`: Coin ID (e.g., "bitcoin", "ethereum", "solana") — use search to find IDs
- `currency`: Fiat currency for prices (usd, cny, eur, jpy, etc.)
- `count`: Number of results for market listing

## Notes

- Free tier: 30 calls/min, no API key required
- Coin IDs are lowercase names (not ticker symbols)
- Use `/search` endpoint to find coin IDs from names/symbols
