package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// NewsArticle represents a news article
type NewsArticle struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	Source      string    `json:"source"`
	PublishedAt time.Time `json:"published_at"`
	ImageURL    string    `json:"image_url,omitempty"`
}

// NewsConfig holds news API configuration
type NewsConfig struct {
	APIKey   string `json:"api_key"`
	Endpoint string `json:"endpoint"`
}

// News is a built-in news aggregation skill
type News struct {
	manifest *skill.Manifest
	config   *NewsConfig
	client   *http.Client
	cache    map[string][]NewsArticle
	cacheMu  sync.RWMutex
}

// NewNews creates a new news skill
func NewNews(config *NewsConfig) *News {
	return &News{
		manifest: &skill.Manifest{
			ID:          "news",
			Name:        "News",
			Version:     "1.0.0",
			Description: "News aggregation and headlines",
			Category:    "information",
			Icon:        "news",
			Tags:        []string{"news", "headlines", "articles"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: headlines, search, sources",
					Required:    true,
				},
				{
					Name:        "category",
					Type:        "string",
					Description: "Category: business, technology, sports, entertainment, health, science",
					Required:    false,
				},
				{
					Name:        "query",
					Type:        "string",
					Description: "Search query",
					Required:    false,
				},
				{
					Name:        "country",
					Type:        "string",
					Description: "Country code (e.g., us, gb, cn)",
					Required:    false,
					Default:     "us",
				},
				{
					Name:        "limit",
					Type:        "number",
					Description: "Number of articles",
					Required:    false,
					Default:     10,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "articles",
					Type:        "array",
					Description: "List of news articles",
				},
			},
		},
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
		cache:  make(map[string][]NewsArticle),
	}
}

// Manifest returns the skill manifest
func (n *News) Manifest() *skill.Manifest {
	return n.manifest
}

// Validate validates the input parameters
func (n *News) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{"headlines": true, "search": true, "sources": true}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	if actionStr == "search" {
		if _, ok := input["query"]; !ok {
			return fmt.Errorf("query is required for search action")
		}
	}

	return nil
}

// Execute executes the news skill
func (n *News) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	switch action {
	case "headlines":
		return n.getHeadlines(ctx, input)
	case "search":
		return n.searchNews(ctx, input)
	case "sources":
		return n.getSources(ctx)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (n *News) getHeadlines(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if n.config == nil || n.config.APIKey == "" {
		return n.getMockHeadlines(input)
	}

	country := "us"
	if c, ok := input["country"].(string); ok {
		country = c
	}

	category := ""
	if cat, ok := input["category"].(string); ok {
		category = cat
	}

	limit := 10
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	endpoint := n.config.Endpoint
	if endpoint == "" {
		endpoint = "https://newsapi.org/v2"
	}

	reqURL := fmt.Sprintf("%s/top-headlines?country=%s&apiKey=%s", endpoint, country, n.config.APIKey)
	if category != "" {
		reqURL += "&category=" + url.QueryEscape(category)
	}
	reqURL += fmt.Sprintf("&pageSize=%d", limit)

	return n.fetchNews(ctx, reqURL, limit)
}

func (n *News) searchNews(ctx context.Context, input map[string]any) (*skill.Result, error) {
	query := input["query"].(string)

	if n.config == nil || n.config.APIKey == "" {
		return skill.NewResult(map[string]any{
			"articles": []NewsArticle{},
			"query":    query,
			"message":  "News API not configured. Please configure API key for actual news search.",
		}), nil
	}

	limit := 10
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	endpoint := n.config.Endpoint
	if endpoint == "" {
		endpoint = "https://newsapi.org/v2"
	}

	reqURL := fmt.Sprintf("%s/everything?q=%s&apiKey=%s&pageSize=%d",
		endpoint, url.QueryEscape(query), n.config.APIKey, limit)

	return n.fetchNews(ctx, reqURL, limit)
}

func (n *News) fetchNews(ctx context.Context, reqURL string, limit int) (*skill.Result, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	var result struct {
		Status   string `json:"status"`
		Articles []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			URL         string `json:"url"`
			Source      struct {
				Name string `json:"name"`
			} `json:"source"`
			PublishedAt string `json:"publishedAt"`
			URLToImage  string `json:"urlToImage"`
		} `json:"articles"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return skill.NewErrorResult(err), nil
	}

	articles := make([]NewsArticle, 0, len(result.Articles))
	for _, a := range result.Articles {
		pubTime, _ := time.Parse(time.RFC3339, a.PublishedAt)
		articles = append(articles, NewsArticle{
			Title:       a.Title,
			Description: a.Description,
			URL:         a.URL,
			Source:      a.Source.Name,
			PublishedAt: pubTime,
			ImageURL:    a.URLToImage,
		})
	}

	return skill.NewResult(map[string]any{
		"articles": articles,
		"count":    len(articles),
	}), nil
}

func (n *News) getMockHeadlines(input map[string]any) (*skill.Result, error) {
	category := "general"
	if cat, ok := input["category"].(string); ok {
		category = cat
	}

	return skill.NewResult(map[string]any{
		"articles": []NewsArticle{},
		"category": category,
		"message":  "News API not configured. Please configure API key for actual news headlines.",
	}), nil
}

func (n *News) getSources(ctx context.Context) (*skill.Result, error) {
	sources := []map[string]string{
		{"id": "bbc-news", "name": "BBC News", "category": "general"},
		{"id": "cnn", "name": "CNN", "category": "general"},
		{"id": "techcrunch", "name": "TechCrunch", "category": "technology"},
		{"id": "the-verge", "name": "The Verge", "category": "technology"},
		{"id": "espn", "name": "ESPN", "category": "sports"},
		{"id": "bloomberg", "name": "Bloomberg", "category": "business"},
	}

	return skill.NewResult(map[string]any{
		"sources": sources,
		"count":   len(sources),
	}), nil
}

// Stocks is a built-in stock market skill
type Stocks struct {
	manifest *skill.Manifest
	config   *StocksConfig
	client   *http.Client
}

// StocksConfig holds stocks API configuration
type StocksConfig struct {
	APIKey   string `json:"api_key"`
	Endpoint string `json:"endpoint"`
}

// NewStocks creates a new stocks skill
func NewStocks(config *StocksConfig) *Stocks {
	return &Stocks{
		manifest: &skill.Manifest{
			ID:          "stocks",
			Name:        "Stocks",
			Version:     "1.0.0",
			Description: "Stock market data and quotes",
			Category:    "information",
			Icon:        "stocks",
			Tags:        []string{"stocks", "market", "finance", "trading"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: quote, search, history",
					Required:    true,
				},
				{
					Name:        "symbol",
					Type:        "string",
					Description: "Stock symbol (e.g., AAPL, GOOGL)",
					Required:    false,
				},
				{
					Name:        "query",
					Type:        "string",
					Description: "Search query for company name",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "quote",
					Type:        "object",
					Description: "Stock quote data",
				},
				{
					Name:        "results",
					Type:        "array",
					Description: "Search results",
				},
			},
		},
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Manifest returns the skill manifest
func (s *Stocks) Manifest() *skill.Manifest {
	return s.manifest
}

// Validate validates the input parameters
func (s *Stocks) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{"quote": true, "search": true, "history": true}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	if actionStr == "quote" || actionStr == "history" {
		if _, ok := input["symbol"]; !ok {
			return fmt.Errorf("symbol is required for %s action", actionStr)
		}
	}

	if actionStr == "search" {
		if _, ok := input["query"]; !ok {
			return fmt.Errorf("query is required for search action")
		}
	}

	return nil
}

// Execute executes the stocks skill
func (s *Stocks) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	switch action {
	case "quote":
		return s.getQuote(ctx, input)
	case "search":
		return s.searchStocks(ctx, input)
	case "history":
		return s.getHistory(ctx, input)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (s *Stocks) getQuote(ctx context.Context, input map[string]any) (*skill.Result, error) {
	symbol := input["symbol"].(string)

	if s.config == nil || s.config.APIKey == "" {
		// Return placeholder data
		return skill.NewResult(map[string]any{
			"symbol":  symbol,
			"message": "Stocks API not configured. Please configure API key for actual stock quotes.",
			"quote": map[string]any{
				"symbol":        symbol,
				"price":         0.0,
				"change":        0.0,
				"changePercent": 0.0,
				"volume":        0,
				"timestamp":     timeutil.NowTime().Format(time.RFC3339),
			},
		}), nil
	}

	// In production, call actual stock API
	return skill.NewResult(map[string]any{
		"symbol":  symbol,
		"message": "Stock quote placeholder",
	}), nil
}

func (s *Stocks) searchStocks(ctx context.Context, input map[string]any) (*skill.Result, error) {
	query := input["query"].(string)

	return skill.NewResult(map[string]any{
		"query":   query,
		"results": []map[string]string{},
		"message": "Stock search placeholder. Configure API for actual search.",
	}), nil
}

func (s *Stocks) getHistory(ctx context.Context, input map[string]any) (*skill.Result, error) {
	symbol := input["symbol"].(string)

	return skill.NewResult(map[string]any{
		"symbol":  symbol,
		"history": []map[string]any{},
		"message": "Stock history placeholder. Configure API for actual history.",
	}), nil
}

// Crypto is a built-in cryptocurrency skill
type Crypto struct {
	manifest *skill.Manifest
	config   *CryptoConfig
	client   *http.Client
}

// CryptoConfig holds crypto API configuration
type CryptoConfig struct {
	APIKey   string `json:"api_key"`
	Endpoint string `json:"endpoint"`
}

// NewCrypto creates a new crypto skill
func NewCrypto(config *CryptoConfig) *Crypto {
	return &Crypto{
		manifest: &skill.Manifest{
			ID:          "crypto",
			Name:        "Crypto",
			Version:     "1.0.0",
			Description: "Cryptocurrency prices and market data",
			Category:    "information",
			Icon:        "crypto",
			Tags:        []string{"crypto", "bitcoin", "ethereum", "blockchain"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: price, list, convert",
					Required:    true,
				},
				{
					Name:        "symbol",
					Type:        "string",
					Description: "Crypto symbol (e.g., BTC, ETH)",
					Required:    false,
				},
				{
					Name:        "currency",
					Type:        "string",
					Description: "Fiat currency (e.g., USD, EUR)",
					Required:    false,
					Default:     "USD",
				},
				{
					Name:        "amount",
					Type:        "number",
					Description: "Amount to convert",
					Required:    false,
				},
				{
					Name:        "from",
					Type:        "string",
					Description: "Source currency for conversion",
					Required:    false,
				},
				{
					Name:        "to",
					Type:        "string",
					Description: "Target currency for conversion",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "price",
					Type:        "object",
					Description: "Crypto price data",
				},
				{
					Name:        "coins",
					Type:        "array",
					Description: "List of cryptocurrencies",
				},
			},
		},
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Manifest returns the skill manifest
func (c *Crypto) Manifest() *skill.Manifest {
	return c.manifest
}

// Validate validates the input parameters
func (c *Crypto) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	validActions := map[string]bool{"price": true, "list": true, "convert": true}
	if !validActions[actionStr] {
		return fmt.Errorf("invalid action: %s", actionStr)
	}

	if actionStr == "price" {
		if _, ok := input["symbol"]; !ok {
			return fmt.Errorf("symbol is required for price action")
		}
	}

	if actionStr == "convert" {
		if _, ok := input["amount"]; !ok {
			return fmt.Errorf("amount is required for convert action")
		}
		if _, ok := input["from"]; !ok {
			return fmt.Errorf("from is required for convert action")
		}
		if _, ok := input["to"]; !ok {
			return fmt.Errorf("to is required for convert action")
		}
	}

	return nil
}

// Execute executes the crypto skill
func (c *Crypto) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	switch action {
	case "price":
		return c.getPrice(ctx, input)
	case "list":
		return c.listCoins(ctx)
	case "convert":
		return c.convert(ctx, input)
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

func (c *Crypto) getPrice(ctx context.Context, input map[string]any) (*skill.Result, error) {
	symbol := input["symbol"].(string)
	currency := "USD"
	if cur, ok := input["currency"].(string); ok {
		currency = cur
	}

	if c.config == nil || c.config.APIKey == "" {
		return skill.NewResult(map[string]any{
			"symbol":   symbol,
			"currency": currency,
			"message":  "Crypto API not configured. Please configure API key for actual prices.",
			"price": map[string]any{
				"symbol":        symbol,
				"price":         0.0,
				"change24h":     0.0,
				"marketCap":     0,
				"volume24h":     0,
				"timestamp":     timeutil.NowTime().Format(time.RFC3339),
			},
		}), nil
	}

	return skill.NewResult(map[string]any{
		"symbol":  symbol,
		"message": "Crypto price placeholder",
	}), nil
}

func (c *Crypto) listCoins(ctx context.Context) (*skill.Result, error) {
	coins := []map[string]string{
		{"symbol": "BTC", "name": "Bitcoin"},
		{"symbol": "ETH", "name": "Ethereum"},
		{"symbol": "BNB", "name": "Binance Coin"},
		{"symbol": "XRP", "name": "Ripple"},
		{"symbol": "ADA", "name": "Cardano"},
		{"symbol": "SOL", "name": "Solana"},
		{"symbol": "DOGE", "name": "Dogecoin"},
		{"symbol": "DOT", "name": "Polkadot"},
		{"symbol": "MATIC", "name": "Polygon"},
		{"symbol": "LTC", "name": "Litecoin"},
	}

	return skill.NewResult(map[string]any{
		"coins": coins,
		"count": len(coins),
	}), nil
}

func (c *Crypto) convert(ctx context.Context, input map[string]any) (*skill.Result, error) {
	var amount float64
	switch v := input["amount"].(type) {
	case float64:
		amount = v
	case int:
		amount = float64(v)
	}

	from := input["from"].(string)
	to := input["to"].(string)

	return skill.NewResult(map[string]any{
		"amount":  amount,
		"from":    from,
		"to":      to,
		"result":  0.0,
		"message": "Crypto conversion placeholder. Configure API for actual conversion.",
	}), nil
}
