# Docker Installation

Run PicoClaw and its companion services (web scraping, video transcription) using Docker Compose.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   picoclaw-net                       │
│                                                     │
│  ┌──────────────┐    ┌──────────────┐               │
│  │  picoclaw     │───▶│  crawl4ai    │  :11235       │
│  │  (gateway or  │    │  (scraper)   │               │
│  │   agent)      │    └──────────────┘               │
│  │              │    ┌──────────────┐               │
│  │              │───▶│ yt-transcript │  :8000        │
│  │              │    │ (captions)    │               │
│  │              │    └──────────────┘               │
│  │              │    ┌──────────────┐               │
│  │              │───▶│ whisper-asr   │  :9000        │
│  │              │    │ (speech-to-  │               │
│  │              │    │  text)        │               │
│  └──────┬───────┘    └──────────────┘               │
│         │                                           │
└─────────┼───────────────────────────────────────────┘
          │
    ┌─────▼─────┐
    │  Volumes   │
    │            │
    │ config.json│ (bind mount, read-only)
    │ memory/    │ (bind mount, host-accessible)
    │ workspace/ │ (named volume)
    └────────────┘
```

## Prerequisites

- Docker Engine 20.10+ and Docker Compose v2
- At least 4 GB RAM available for containers
- An LLM API key (OpenAI, Anthropic, etc.)

## Quick Start

### 1. Clone and enter the repository

```bash
git clone https://github.com/helio1973/picoclaw.git
cd picoclaw
```

### 2. Create your configuration

```bash
cp config/config.example.json config/config.json
```

Edit `config/config.json` and set your API keys. Make sure `allowed_hosts` includes the companion services:

```json
{
  "tools": {
    "web": {
      "allowed_hosts": ["crawl4ai", "yt-transcript", "whisper-asr"]
    }
  }
}
```

### 3. (Optional) Customize environment variables

```bash
cp docker/.env.example .env
```

Only needed if you want to change default ports, Whisper model size, or memory vault path. See [Environment Variables](#environment-variables) below.

### 4. Start the services

```bash
# Start companion services (scraper, transcription)
docker compose up -d crawl4ai yt-transcript whisper-asr

# Wait for services to be healthy
docker compose logs -f crawl4ai yt-transcript whisper-asr
```

### 5. Start PicoClaw

**Gateway mode** (long-running bot, connects to Telegram/Discord/etc.):

```bash
docker compose --profile gateway up -d picoclaw-gateway
```

**Agent mode** (one-shot query):

```bash
docker compose --profile agent run --rm picoclaw-agent -m "Hello, what can you do?"
```

### 6. Verify everything is running

```bash
# Check all containers
docker compose ps

# Crawl4AI health check
curl http://localhost:11235/monitor/health

# YouTube Transcript API
curl http://localhost:8200/docs

# Whisper ASR
curl http://localhost:9000/docs
```

## Services

### PicoClaw (gateway / agent)

The main AI agent. Runs in two modes:

| Mode | Profile | Command | Description |
|------|---------|---------|-------------|
| Gateway | `gateway` | `docker compose --profile gateway up -d` | Long-running bot with channel integrations |
| Agent | `agent` | `docker compose --profile agent run --rm picoclaw-agent -m "..."` | One-shot query, exits when done |

**Exposed port:** 18790 (gateway health check)

### Crawl4AI

LLM-ready web scraping service using a headless Chromium browser. Replaces simple HTTP fetches for JavaScript-heavy sites, SPAs, and dynamic content.

| Property | Value |
|----------|-------|
| Image | `unclecode/crawl4ai:latest` |
| Internal URL | `http://crawl4ai:11235` |
| Host port | 11235 (configurable) |
| Docs | `http://localhost:11235/docs` |
| Playground | `http://localhost:11235/playground` |

### YouTube Transcript API

Extracts existing captions and subtitles from YouTube videos. Instant results, no compute needed.

| Property | Value |
|----------|-------|
| Image | `yoanbernabeu/youtubetranscriptapi:latest` |
| Internal URL | `http://yt-transcript:8000` |
| Host port | 8200 (configurable) |
| Docs | `http://localhost:8200/docs` |

### Whisper ASR

Speech-to-text service using Faster Whisper. Transcribes audio files in 99+ languages. Used as a fallback when YouTube captions are not available.

| Property | Value |
|----------|-------|
| Image | `onerahmet/openai-whisper-asr-webservice:latest` |
| Internal URL | `http://whisper-asr:9000` |
| Host port | 9000 (configurable) |
| Docs | `http://localhost:9000/docs` |

## Environment Variables

All variables are optional. Defaults are shown below.

| Variable | Default | Description |
|----------|---------|-------------|
| `MEMORY_VAULT_PATH` | `./data/memory` | Host path for the memory vault bind mount |
| `CRAWL4AI_HOST_PORT` | `11235` | Crawl4AI port on the host |
| `CRAWL4AI_MAX_CONCURRENT` | `5` | Max concurrent crawl tasks |
| `CRAWL4AI_API_TOKEN` | _(empty)_ | Optional API token for Crawl4AI auth |
| `YT_TRANSCRIPT_HOST_PORT` | `8200` | YouTube Transcript API port on the host |
| `WHISPER_HOST_PORT` | `9000` | Whisper ASR port on the host |
| `WHISPER_MODEL` | `base` | Whisper model: `tiny`, `base`, `small`, `medium`, `large-v3` |

### Whisper model sizes

| Model | Size | RAM | Speed (CPU) | Quality |
|-------|------|-----|-------------|---------|
| `tiny` | 39 MB | ~1 GB | Fast | Basic |
| `base` | 142 MB | ~1 GB | Good | Good |
| `small` | 466 MB | ~2 GB | Moderate | Better |
| `medium` | 1.5 GB | ~4 GB | Slow | Great |
| `large-v3` | 3.1 GB | ~6 GB | Very slow | Best |

For CPU-only setups, `base` or `small` is recommended. Use `large-v3` only with a GPU.

## Volumes and Data

| Volume | Mount | Purpose |
|--------|-------|---------|
| `picoclaw-workspace` | Named volume | Agent workspace (skills, files) |
| `crawl4ai-data` | Named volume | Crawl4AI browser cache |
| `whisper-cache` | Named volume | Downloaded Whisper models |
| `config/config.json` | Bind mount (read-only) | PicoClaw configuration |
| `${MEMORY_VAULT_PATH}` | Bind mount | Memory vault (accessible from host) |

The memory vault is bind-mounted so its contents are directly accessible from the host filesystem at the configured path (default: `./data/memory`).

## Networking

All services communicate over the `picoclaw-net` bridge network using container names as hostnames. The SSRF allowlist in `config.json` must include these hostnames so PicoClaw can reach them via `web_fetch`:

```json
"allowed_hosts": ["crawl4ai", "yt-transcript", "whisper-asr"]
```

## Common Operations

### View logs

```bash
# All services
docker compose logs -f

# Specific service
docker compose logs -f picoclaw-gateway
```

### Rebuild after code changes

```bash
docker compose build picoclaw-gateway picoclaw-agent
docker compose --profile gateway up -d picoclaw-gateway
```

### Stop everything

```bash
docker compose --profile gateway down
```

### Reset all data

```bash
docker compose --profile gateway down -v
```

> **Warning:** This deletes all named volumes including the workspace, Whisper model cache, and Crawl4AI data. The memory vault (bind mount) is not affected.

### Update companion service images

```bash
docker compose pull crawl4ai yt-transcript whisper-asr
docker compose up -d crawl4ai yt-transcript whisper-asr
```

## Skills

The Docker image includes built-in skills that are automatically synced into the workspace on first startup:

| Skill | Description |
|-------|-------------|
| `deep-scraper` | Web scraping via Crawl4AI |
| `youtube-transcriber` | Video transcription via YouTube Transcript API + Whisper ASR |

Skills are synced by the entrypoint script (`docker/entrypoint.sh`). Existing skills in the workspace volume are never overwritten.

## Troubleshooting

### Services can't reach each other

Verify all services are on the same network:

```bash
docker network inspect picoclaw_picoclaw-net
```

### Whisper ASR is slow

The default `base` model is optimized for CPU. If transcription is too slow:

1. Try the `tiny` model: set `WHISPER_MODEL=tiny` in `.env`
2. For better quality with GPU, use `large-v3` with the GPU image variant

### PicoClaw can't reach companion services

Check that `allowed_hosts` in `config/config.json` includes the service hostnames. Without this, the SSRF protection blocks requests to private Docker network IPs.

### Built-in skills are missing

The entrypoint script syncs skills on startup. Check the logs:

```bash
docker compose logs picoclaw-gateway | grep "Installed built-in skill"
```

If skills are still missing, remove the workspace volume and restart:

```bash
docker compose --profile gateway down
docker volume rm picoclaw_picoclaw-workspace
docker compose --profile gateway up -d
```
