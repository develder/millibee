# Docker Installation

Run MilliBee and its companion services (web scraping, video transcription) using Docker Compose.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   millibee-net                       │
│                                                     │
│  ┌──────────────┐    ┌──────────────┐               │
│  │  millibee     │───▶│  crawl4ai    │  :11235       │
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
    │ workspace/ │ (bind mount, host-accessible)
    └────────────┘
```

## Prerequisites

- Docker Engine 20.10+ and Docker Compose v2
- At least 4 GB RAM available for containers
- An LLM API key (OpenAI, Anthropic, etc.)

## Quick Start

### 1. Clone and enter the repository

```bash
git clone https://github.com/develder/millibee.git
cd millibee
```

### 2. Create your configuration

```bash
cp config/config.example.json config/config.json
```

Edit `config/config.json` and set your API keys. Enable the sidecar tools you want to use:

```json
{
  "tools": {
    "sidecars": {
      "crawl4ai":      { "enabled": true },
      "yt_transcript": { "enabled": true },
      "whisper_asr":   { "enabled": true }
    }
  }
}
```

This registers the `deep_scrape`, `youtube_transcript`, and `transcribe_audio` native tools in the agent's toolset.

### 3. (Optional) Customize environment variables

```bash
cp docker/.env.example docker/.env
```

Only needed if you want to change default ports, Whisper model size, or workspace path. See [Environment Variables](#environment-variables) below.

### 4. Start the services

```bash
# Start companion services (scraper, transcription)
docker compose -f docker/docker-compose.yml up -d crawl4ai yt-transcript whisper-asr

# Wait for services to be healthy
docker compose -f docker/docker-compose.yml logs -f crawl4ai yt-transcript whisper-asr
```

### 5. Start MilliBee

**Gateway mode** (long-running bot, connects to Telegram/Discord/etc.):

```bash
docker compose -f docker/docker-compose.yml --profile gateway up -d millibee-gateway
```

**Agent mode** (one-shot query):

```bash
docker compose -f docker/docker-compose.yml --profile agent run --rm millibee-agent -m "Hello, what can you do?"
```

### 6. Verify everything is running

```bash
# Check all containers
docker compose -f docker/docker-compose.yml ps

# Crawl4AI health check
curl http://localhost:11235/monitor/health

# YouTube Transcript API
curl http://localhost:8200/docs

# Whisper ASR
curl http://localhost:9000/docs
```

## Services

### MilliBee (gateway / agent)

The main AI agent. Runs in two modes:

| Mode | Profile | Command | Description |
|------|---------|---------|-------------|
| Gateway | `gateway` | `docker compose -f docker/docker-compose.yml --profile gateway up -d` | Long-running bot with channel integrations |
| Agent | `agent` | `docker compose -f docker/docker-compose.yml --profile agent run --rm millibee-agent -m "..."` | One-shot query, exits when done |

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
| `WORKSPACE_PATH` | `./data/workspace` | Host path for the workspace bind mount |
| `SSH_HOST_PORT` | `2222` | SSH TUI port on the host |
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
| `config/config.json` | Bind mount (read-only) | MilliBee configuration |
| `${WORKSPACE_PATH}` | Bind mount | Workspace: memory vault, skills, projects |
| `crawl4ai-data` | Named volume | Crawl4AI browser cache |
| `whisper-cache` | Named volume | Downloaded Whisper models |

The entire workspace is bind-mounted to the host (default: `./data/workspace`). This means all workspace contents — memory vault, skills, and any project files — are directly accessible from the host filesystem. You can edit skills in your IDE, back up the workspace with standard tools, and inspect memory vault contents without `docker exec`.

The workspace directory structure:

```
data/workspace/
├── memory/       # Memory vault (persistent notes)
├── skills/       # Installed skills
└── ...           # Any other agent-created files
```

## Networking

All services communicate over the `millibee-net` bridge network using container names as hostnames. The native sidecar tools (`deep_scrape`, `youtube_transcript`, `transcribe_audio`) call these services directly using the configured `base_url` — no SSRF allowlist needed for them.

If you also want to reach the sidecar services via `web_fetch` (e.g., from a skill), add them to the allowlist:

```json
"allowed_hosts": ["crawl4ai", "yt-transcript", "whisper-asr"]
```

## Common Operations

### View logs

```bash
# All services
docker compose -f docker/docker-compose.yml logs -f

# Specific service
docker compose -f docker/docker-compose.yml logs -f millibee-gateway
```

### Rebuild after code changes

```bash
docker compose -f docker/docker-compose.yml build millibee-gateway millibee-agent
docker compose -f docker/docker-compose.yml --profile gateway up -d millibee-gateway
```

### Stop everything

```bash
docker compose -f docker/docker-compose.yml --profile gateway down
```

### Reset all data

```bash
docker compose -f docker/docker-compose.yml --profile gateway down -v
```

> **Warning:** This deletes all named volumes (Whisper model cache, Crawl4AI data). The workspace bind mount on the host is not affected.

### Update companion service images

```bash
docker compose -f docker/docker-compose.yml pull crawl4ai yt-transcript whisper-asr
docker compose -f docker/docker-compose.yml up -d crawl4ai yt-transcript whisper-asr
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
docker network inspect millibee_millibee-net
```

### Whisper ASR is slow

The default `base` model is optimized for CPU. If transcription is too slow:

1. Try the `tiny` model: set `WHISPER_MODEL=tiny` in `.env`
2. For better quality with GPU, use `large-v3` with the GPU image variant

### MilliBee can't reach companion services

Check that `allowed_hosts` in `config/config.json` includes the service hostnames. Without this, the SSRF protection blocks requests to private Docker network IPs.

### Built-in skills are missing

The entrypoint script syncs skills on startup. Check the logs:

```bash
docker compose -f docker/docker-compose.yml logs millibee-gateway | grep "Installed built-in skill"
```

If skills are still missing, remove the skills directory from the workspace and restart:

```bash
rm -rf data/workspace/skills
docker compose -f docker/docker-compose.yml --profile gateway restart millibee-gateway
```
