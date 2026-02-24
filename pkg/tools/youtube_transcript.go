package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// YouTubeTranscriptTool extracts transcripts from YouTube videos via an
// external transcript service. It supports both direct video IDs and full
// YouTube URLs, and can request transcripts in different languages.
type YouTubeTranscriptTool struct {
	baseURL string
	client  *http.Client
}

// NewYouTubeTranscriptTool creates a new YouTubeTranscriptTool that calls the
// transcript service at baseURL. If timeoutSecs is <= 0, a default of 30
// seconds is used.
func NewYouTubeTranscriptTool(baseURL string, timeoutSecs int) *YouTubeTranscriptTool {
	if timeoutSecs <= 0 {
		timeoutSecs = 30
	}
	return &YouTubeTranscriptTool{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: time.Duration(timeoutSecs) * time.Second,
		},
	}
}

// Name returns the tool name.
func (t *YouTubeTranscriptTool) Name() string {
	return "youtube_transcript"
}

// Description returns a human-readable description of the tool.
func (t *YouTubeTranscriptTool) Description() string {
	return "Extract transcript or captions from a YouTube video. Returns the full transcript text with timestamps. Supports auto-generated and manual captions in multiple languages."
}

// Parameters returns the JSON Schema for this tool's parameters.
func (t *YouTubeTranscriptTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "YouTube video URL",
			},
			"video_id": map[string]any{
				"type":        "string",
				"description": "YouTube video ID (one of url or video_id required)",
			},
			"language": map[string]any{
				"type":        "string",
				"description": "Language code (default \"en\")",
			},
		},
	}
}

// Execute fetches the transcript for the given YouTube video.
func (t *YouTubeTranscriptTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	videoID, _ := args["video_id"].(string)
	rawURL, _ := args["url"].(string)
	language, _ := args["language"].(string)

	if language == "" {
		language = "en"
	}

	// Resolve video ID
	if videoID == "" {
		if rawURL == "" {
			return ErrorResult("either url or video_id is required")
		}
		var err error
		videoID, err = extractVideoID(rawURL)
		if err != nil {
			return ErrorResult(fmt.Sprintf("failed to extract video ID from url: %s", err))
		}
	}

	// Build request URL
	reqURL := fmt.Sprintf("%s/transcript?video_id=%s&language=%s",
		t.baseURL,
		url.QueryEscape(videoID),
		url.QueryEscape(language),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create request: %s", err))
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("transcript service request failed: %s", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ErrorResult(fmt.Sprintf("transcript service returned status %d: %s", resp.StatusCode, string(body)))
	}

	// Parse response segments
	var segments []struct {
		Text     string  `json:"text"`
		Start    float64 `json:"start"`
		Duration float64 `json:"duration"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&segments); err != nil {
		return ErrorResult(fmt.Sprintf("failed to parse transcript response: %s", err))
	}

	// Join segment texts
	texts := make([]string, len(segments))
	for i, seg := range segments {
		texts[i] = seg.Text
	}
	transcript := strings.Join(texts, " ")

	return &ToolResult{
		ForLLM:  transcript,
		ForUser: fmt.Sprintf("Transcript for %s (%d chars)", videoID, len(transcript)),
	}
}

// extractVideoID extracts a YouTube video ID from various URL formats.
// It uses net/url for parsing rather than regex.
func extractVideoID(rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("empty URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.ToLower(parsed.Hostname())

	// youtu.be short URLs: path is /{video_id}
	if host == "youtu.be" {
		id := strings.TrimPrefix(parsed.Path, "/")
		if id == "" {
			return "", fmt.Errorf("no video ID in short URL")
		}
		return id, nil
	}

	// Must be a youtube.com host
	if host != "www.youtube.com" && host != "youtube.com" {
		return "", fmt.Errorf("not a YouTube URL: %s", host)
	}

	// /watch?v={id}
	if parsed.Path == "/watch" {
		id := parsed.Query().Get("v")
		if id == "" {
			return "", fmt.Errorf("no video ID in watch URL")
		}
		return id, nil
	}

	// /embed/{id}, /shorts/{id}, /live/{id}
	pathParts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
	if len(pathParts) == 2 {
		prefix := pathParts[0]
		id := pathParts[1]
		if (prefix == "embed" || prefix == "shorts" || prefix == "live") && id != "" {
			return id, nil
		}
	}

	return "", fmt.Errorf("could not extract video ID from URL: %s", rawURL)
}
