---
name: deep-scraper
description: "Deep web scraping via Crawl4AI. Scrape single pages or crawl entire sites with a headless browser. Returns LLM-ready Markdown with clean content extraction."
---

# Deep Scraper Skill

Use the Crawl4AI service to scrape web pages rendered by a full headless browser (Chromium). This is useful for JavaScript-heavy sites, SPAs, and content behind client-side rendering that simple HTTP fetches cannot handle. Crawl4AI returns clean, LLM-ready Markdown.

The service is available at `${CRAWL4AI_BASE_URL}` (default: `http://crawl4ai:11235`).

## Single Page Scrape

Scrape a single URL and get Markdown content:

```bash
curl -s -X POST "${CRAWL4AI_BASE_URL:-http://crawl4ai:11235}/crawl" \
  -H "Content-Type: application/json" \
  -d '{
    "urls": ["https://example.com"],
    "crawler_config": {
      "type": "CrawlerRunConfig",
      "params": {"cache_mode": "bypass"}
    }
  }'
```

**Response** (relevant fields):
```json
{
  "results": [
    {
      "url": "https://example.com",
      "markdown": "# Page Title\n\nClean markdown content...",
      "fit_markdown": "Filtered markdown with only relevant content...",
      "metadata": {"title": "Page Title", "description": "..."},
      "success": true
    }
  ]
}
```

Use `fit_markdown` for concise content (noise removed) or `markdown` for the full page.

## Get Only Markdown

For a lightweight request that returns just the Markdown:

```bash
curl -s -X POST "${CRAWL4AI_BASE_URL:-http://crawl4ai:11235}/md" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com"}'
```

## Screenshot

Capture a full-page screenshot:

```bash
curl -s -X POST "${CRAWL4AI_BASE_URL:-http://crawl4ai:11235}/screenshot" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com"}' --output screenshot.png
```

## Deep Scrape (Multi-page Crawl)

Use the async job API for multi-page crawls:

```bash
# Submit crawl job
JOB=$(curl -s -X POST "${CRAWL4AI_BASE_URL:-http://crawl4ai:11235}/crawl/job" \
  -H "Content-Type: application/json" \
  -d '{
    "urls": ["https://docs.example.com"],
    "crawler_config": {
      "type": "CrawlerRunConfig",
      "params": {
        "cache_mode": "bypass",
        "deep_crawl_strategy": {
          "type": "BFSDeepCrawlStrategy",
          "params": {"max_depth": 2, "max_pages": 10}
        }
      }
    }
  }')

# Extract task ID
TASK_ID=$(echo "$JOB" | jq -r '.task_id')

# Poll for results
curl -s "${CRAWL4AI_BASE_URL:-http://crawl4ai:11235}/job/$TASK_ID"
```

**Parameters for deep crawl:**
- `max_depth` (default 2): How many link levels deep to follow
- `max_pages` (default 10): Maximum number of pages to crawl

## Execute JavaScript

Run custom JavaScript on a page before extraction:

```bash
curl -s -X POST "${CRAWL4AI_BASE_URL:-http://crawl4ai:11235}/execute_js" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com/app",
    "js_code": "document.querySelector(\"button.load-more\").click(); await new Promise(r => setTimeout(r, 2000));"
  }'
```

## Configuration

Add `crawl4ai` to the SSRF allowlist in `config.json` so the agent can reach the service on the Docker network:

```json
{
  "tools": {
    "web": {
      "allowed_hosts": ["crawl4ai"]
    }
  }
}
```

## Tips

- Use `/crawl` for single-page content with full metadata
- Use `/md` for quick Markdown-only extraction
- Use `/crawl/job` for deep multi-page crawls (async)
- Prefer `fit_markdown` over `markdown` — it filters out navigation, footers, and other noise
- The `/playground` endpoint (exposed on host) provides an interactive web UI for testing
- The `/monitor` endpoint shows real-time browser pool and request stats
