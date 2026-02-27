---
name: youtube-transcriber
description: "Transcribe YouTube videos using captions API (fast) or Whisper ASR speech-to-text (fallback). Returns full transcript text."
---

# YouTube Transcriber Skill

Transcribe YouTube videos in two ways:

1. **YouTube Transcript API** — extracts existing captions/subtitles (instant, preferred)
2. **Whisper ASR** — downloads audio and runs speech-to-text (slower, works on any video)

Always try the Transcript API first. Only fall back to Whisper if no captions are available.

Use the `web_fetch` tool for all HTTP requests (curl is not available).

## Step 1: Extract Video ID

Extract the video ID from the YouTube URL:

- `https://www.youtube.com/watch?v=dQw4w9WgXcQ` → `dQw4w9WgXcQ`
- `https://youtu.be/dQw4w9WgXcQ` → `dQw4w9WgXcQ`
- `https://www.youtube.com/embed/dQw4w9WgXcQ` → `dQw4w9WgXcQ`

## Step 2: Try YouTube Transcript API (fast)

```
web_fetch(url="http://yt-transcript:8000/transcript?video_id=VIDEO_ID&language=en")
```

With specific language:

```
web_fetch(url="http://yt-transcript:8000/transcript?video_id=VIDEO_ID&language=nl")
```

List available transcript languages for a video:

```
web_fetch(url="http://yt-transcript:8000/transcripts?video_id=VIDEO_ID")
```

Get transcript in SRT format:

```
web_fetch(url="http://yt-transcript:8000/transcript?video_id=VIDEO_ID&language=en&format=srt")
```

**Response** (JSON format):
```json
[
  {"text": "Hello everyone", "start": 0.0, "duration": 2.5},
  {"text": "Welcome to this video", "start": 2.5, "duration": 3.0}
]
```

If this returns an error (no captions available), proceed to Step 3.

## Step 3: Whisper ASR Fallback (speech-to-text)

When no captions exist, download the audio and send it to Whisper ASR via exec:

```bash
# Download audio from YouTube using yt-dlp
yt-dlp -x --audio-format mp3 -o /tmp/yt_audio.mp3 "https://www.youtube.com/watch?v=VIDEO_ID"

# Transcribe with Whisper ASR (multipart upload via Python)
python3 -c "
import requests
with open('/tmp/yt_audio.mp3', 'rb') as f:
    r = requests.post('http://whisper-asr:9000/asr?output=json&language=en', files={'audio_file': f})
    print(r.text)
"

# Clean up
rm -f /tmp/yt_audio.mp3
```

With automatic language detection:

```bash
python3 -c "
import requests
with open('/tmp/yt_audio.mp3', 'rb') as f:
    r = requests.post('http://whisper-asr:9000/asr?output=json', files={'audio_file': f})
    print(r.text)
"
```

**Output formats:** `txt`, `json`, `vtt`, `srt`, `tsv` (change `output=` parameter)

## Configuration

Add the service hostnames to the SSRF allowlist in `config.json`:

```json
{
  "tools": {
    "web": {
      "allowed_hosts": ["yt-transcript", "whisper-asr"]
    }
  }
}
```

## Tips

- The Transcript API is instant and free — always try it first
- Whisper `base` model is good enough for most videos and runs fine on CPU
- For better quality, set `WHISPER_MODEL=large-v3` (requires more RAM, GPU recommended)
- Whisper supports 99+ languages with automatic detection
- For long videos (>1 hour), Whisper may take several minutes on CPU
- The Whisper ASR Swagger docs are available at `http://localhost:9000/docs`
