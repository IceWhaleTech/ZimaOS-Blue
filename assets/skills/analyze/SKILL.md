# Analyze

Deep-dive analysis tool. Gathers data from URLs and web searches, then generates a comprehensive HTML report with statistics, insights, and visualizations.

## How to Send

Direct call (always use `--json` for structured output):

```bash
analyze topic="XXX community feedback" urls='["https://reddit.com/r/minilab/..."]' --json
```

## Commands

### analyze

Full analysis pipeline. Scrapes URLs via browser, runs web searches, then uses LLM to extract structured data and generate a self-contained HTML report.

**Required:** `topic`

**Optional:**
- `urls` — array of URLs to scrape for content (max 5)
- `search_queries` — array of web search queries for additional data (max 3)
- `text` — direct text content to include in analysis
- `lang` — output language (`zh-CN`, `en-US`, default: `zh-CN`)

```bash
analyze topic="Product feedback analysis" urls='["https://example.com/reviews"]' search_queries='["product reviews 2026"]' --json
```

```bash
analyze topic="Survey results" text="..." lang=en-US --json
```

## Pipeline

1. **Data Collection** — parallel URL scraping (browser) + web search + direct text
2. **LLM Analysis** — extracts statistics, themes, quotes, insights, recommendations as structured JSON
3. **Report Generation** — LLM generates HTML body using predefined CSS template
4. **Output** — saves self-contained HTML to `{mediaDir}/analyze/{uuid}.html`, returns URL

## Report Sections

The generated HTML report includes:

| Section | Components |
|---------|-----------|
| Hero | Title, summary, key stats |
| Overview | Stat boxes with color coding |
| Themes | Tag cloud + progress bars |
| Key Findings | Insight boxes (strength/opportunity/challenge/risk) |
| Voices | Quote cards with sentiment coloring |
| Recommendations | Priority-ranked cards |
| Summary | Data table |

## Error Response

All commands return `status=error` with an `error` message on failure.

Common errors:
- `topic is required` — missing `topic` parameter
- `LLM bridge not available` — LLM service not configured
- `no data collected` — no URLs, search queries, or text provided

## Example Triggers

- "Analyze Reddit feedback about ZimaBoard"
- "帮我分析一下这个产品的用户评价"
- "Generate a report on community discussions about ZimaOS"
- "分析这段文本并生成报告"
