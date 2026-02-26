<div align="center">

  <img src="assets/millibee-logo.png" alt="MilliBee" width="400">

  <h1>MilliBee</h1>
  <p><i>She ships.</i></p>

  <h3>Lean AI Coding Assistant &middot; Dockerized &middot; SSH TUI &middot; Security-First</h3>

  <p>
    <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go">
    <img src="https://img.shields.io/badge/Docker-ready-2496ED?style=flat&logo=docker&logoColor=white" alt="Docker">
    <img src="https://img.shields.io/badge/RAM-<10MB-ff69b4" alt="Memory">
    <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
  </p>

</div>

---

**MilliBee** is a lean, dockerized AI coding assistant built in Go. Single binary, < 10MB RAM, boots in under a second.

Started as a fork of [PicoClaw](https://github.com/sipeed/picoclaw). Stripped the hardware marketing, added security hardening, a memory vault, SSH-accessible TUI, native git tools, and streaming responses. The result: a practical self-hosted coding companion you SSH into from anywhere on your network.

## Niche

MilliBee is not trying to be Claude Code or Cursor. She's the **self-hosted, always-on AI assistant** that runs on your home server or NAS:

- **SSH in from any device** — laptop, tablet, phone terminal. No browser, no desktop app, no subscriptions.
- **Your keys, your data** — config and memory vault live on your machine. Nothing leaves your network except API calls.
- **Multi-channel** — same assistant answers on Telegram, SSH, and Console. One config, one brain.
- **Tiny footprint** — runs alongside your other Docker services without hogging resources.

## What MilliBee Adds

Everything below was built on top of PicoClaw's foundation:

| Feature | Description |
|---------|-------------|
| **Security hardening** | AES-256-GCM encrypted credentials at rest, exec safety guards, SSRF protection, rate limiting, input validation middleware |
| **Memory vault** | Persistent notes the agent can save, search, and recall across sessions. Nightly index rebuild. Wikilink support. |
| **SSH TUI channel** | Bubble Tea chat interface served over SSH via [Wish](https://github.com/charmbracelet/wish). `ssh user@host -p 2222` and you're in. |
| **13 native git tools** | status, diff, log, show, branch, commit, add, reset, checkout, pull, merge, stash, push. No shell injection, configurable push policy. |
| **Deep scrape** | Full-page web scraping via Crawl4AI sidecar. JavaScript rendering, markdown extraction. |
| **YouTube transcripts** | Extract transcripts from YouTube videos via sidecar API. |
| **Audio transcription** | Speech-to-text via Whisper ASR sidecar. |
| **Streaming output** | Tokens appear as the LLM thinks. Anthropic and OpenAI-compatible providers. |
| **Native Anthropic SDK** | Direct Anthropic API integration (not OpenAI-compat shim). |
| **Docker services** | Compose stack with Crawl4AI, Whisper ASR, YouTube Transcript API sidecars. |

## Quick Start

### Docker (recommended)

```bash
git clone https://github.com/develder/millibee.git
cd millibee

cp config/config.example.json config/config.json
# Edit config.json — set your API key and SSH password

docker compose -f docker/docker-compose.yml --profile gateway up -d

# Connect via SSH
ssh localhost -p 2222
```

One-shot mode:

```bash
docker compose -f docker/docker-compose.yml run --rm millibee-agent -m "Explain this repo's architecture"
```

### From Source

```bash
git clone https://github.com/develder/millibee.git
cd millibee
make build

alias milli='./build/millibee'

milli onboard
milli agent -m "Hello"
```

## Configuration

Config lives at `~/.millibee/config.json`.

### Minimal Config

```json
{
  "model_list": [
    {
      "model_name": "claude-sonnet-4.6",
      "model": "anthropic/claude-sonnet-4.6",
      "api_key": "sk-ant-your-key"
    }
  ],
  "agents": {
    "defaults": {
      "model": "claude-sonnet-4.6",
      "workspace": "~/.millibee/workspace"
    }
  }
}
```

### SSH Channel Config

```json
{
  "channels": {
    "ssh": {
      "enabled": true,
      "address": "0.0.0.0:2222",
      "password": "your-password"
    }
  }
}
```

Connect with: `ssh anyuser@your-host -p 2222`

### Git Tools Config

Git tools are enabled by default. Push is disabled by default (opt-in for safety):

```json
{
  "tools": {
    "git": {
      "enabled": true,
      "allow_push": false
    }
  }
}
```

### Providers

| Vendor | Prefix | Protocol |
|--------|--------|----------|
| Anthropic | `anthropic/` | Anthropic |
| OpenAI | `openai/` | OpenAI |
| DeepSeek | `deepseek/` | OpenAI |
| Groq | `groq/` | OpenAI |
| Ollama | `ollama/` | OpenAI |
| OpenRouter | `openrouter/` | OpenAI |
| Cerebras | `cerebras/` | OpenAI |
| Qwen | `qwen/` | OpenAI |
| Gemini | `gemini/` | OpenAI |
| VLLM | `vllm/` | OpenAI |

Use `vendor/model` format in `model_list`. Custom endpoints via `api_base`.

### Chat Channels

<details>
<summary><b>SSH TUI</b> (recommended)</summary>

Built-in Bubble Tea chat interface over SSH. Full markdown rendering, streaming, and a spinner.

```json
{
  "channels": {
    "ssh": {
      "enabled": true,
      "address": "0.0.0.0:2222",
      "password": "secret"
    }
  }
}
```

```bash
ssh user@your-host -p 2222
```

</details>

<details>
<summary><b>Telegram</b></summary>

1. Talk to `@BotFather`, create bot, copy token
2. Add to config:

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "allow_from": ["YOUR_USER_ID"]
    }
  }
}
```

3. `milli gateway`

</details>


## Architecture

```
milli gateway                    ssh user@host -p 2222
     │                                  │
     ▼                                  ▼
 ChannelManager ◄── SSH Channel (Wish + Bubble Tea)
     │               Telegram
     │               Console
     ▼
 MessageBus ──► AgentLoop ──► ToolRegistry
                    │          (13 git tools
                    ▼           + file ops
              LLMProvider       + exec
              (Anthropic        + memory vault
               OpenAI-compat    + deep scrape
               streaming)       + web search)
```

## CLI

| Command | Description |
|---------|-------------|
| `milli onboard` | Initialize config & workspace |
| `milli agent -m "..."` | One-shot chat |
| `milli agent` | Interactive CLI |
| `milli gateway` | Start multi-channel gateway (includes SSH TUI) |
| `milli status` | Show status |
| `milli cron list` | List scheduled jobs |

## Security

- **Encrypted credentials** — AES-256-GCM at rest for API keys and tokens
- **Workspace sandbox** — file tools restricted to workspace by default
- **Exec guards** — blocks `rm -rf`, `format`, `dd`, fork bombs, etc.
- **SSRF protection** — web_fetch validates URLs against internal networks
- **Rate limiting** — per-tool rate limits and concurrency caps
- **Git push opt-in** — `allow_push: false` by default
- **No shell interpolation** — git tools use `exec.Command("git", ...)` directly

## Workspace Layout

```
~/.millibee/workspace/
├── sessions/          # Conversation history
├── memory/            # Persistent memory vault
├── state/             # Runtime state
├── cron/              # Scheduled jobs
├── skills/            # Custom skills
├── AGENTS.md          # Agent behavior
├── IDENTITY.md        # Agent identity
└── USER.md            # User preferences
```

## License

MIT

---

<div align="center">
  <i>She's small. She's fast. She ships.</i>
</div>
