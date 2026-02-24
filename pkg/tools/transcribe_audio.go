package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TranscribeAudioTool transcribes audio files to text using a Whisper ASR API.
// It supports 99+ languages with automatic detection and multiple output formats.
type TranscribeAudioTool struct {
	baseURL string
	client  *http.Client
}

// NewTranscribeAudioTool creates a new TranscribeAudioTool.
// baseURL is the Whisper ASR service URL. timeoutSecs defaults to 300 if <= 0.
func NewTranscribeAudioTool(baseURL string, timeoutSecs int) *TranscribeAudioTool {
	if timeoutSecs <= 0 {
		timeoutSecs = 300
	}
	return &TranscribeAudioTool{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: time.Duration(timeoutSecs) * time.Second,
		},
	}
}

// Name returns the tool name.
func (t *TranscribeAudioTool) Name() string {
	return "transcribe_audio"
}

// Description returns a human-readable description of the tool.
func (t *TranscribeAudioTool) Description() string {
	return "Transcribe an audio file to text using speech-to-text (Whisper). Supports 99+ languages with automatic detection. For YouTube videos, use youtube_transcript instead."
}

// Parameters returns the JSON schema for this tool's parameters.
func (t *TranscribeAudioTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Path to the audio file",
			},
			"language": map[string]any{
				"type":        "string",
				"description": "Language code (e.g. \"en\", \"nl\"). Omit for auto-detection",
			},
			"output": map[string]any{
				"type":        "string",
				"description": "Output format - \"txt\", \"json\", \"srt\", \"vtt\" (default \"txt\")",
			},
		},
		"required": []string{"file_path"},
	}
}

// Execute transcribes the audio file at the given path using the Whisper ASR API.
func (t *TranscribeAudioTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	// Extract file_path (required)
	filePath, ok := args["file_path"].(string)
	if !ok || filePath == "" {
		return ErrorResult("file_path is required")
	}

	// Check for YouTube URLs
	if strings.Contains(filePath, "youtube.com") || strings.Contains(filePath, "youtu.be") {
		return ErrorResult("For YouTube videos, use the youtube_transcript tool instead. If no captions are available, download the audio first with the exec tool: yt-dlp -x --audio-format mp3 -o /tmp/audio.mp3 \"<url>\", then pass the file path here.")
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to open file: %v", err))
	}
	defer file.Close()

	// Get optional parameters
	language, _ := args["language"].(string)
	output, _ := args["output"].(string)
	if output == "" {
		output = "txt"
	}

	// Build multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("audio_file", filepath.Base(filePath))
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to create multipart form: %v", err))
	}
	if _, err = io.Copy(part, file); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to read file: %v", err))
	}
	writer.Close()

	// Build URL with query params
	url := fmt.Sprintf("%s/asr?output=%s", t.baseURL, output)
	if language != "" {
		url += "&language=" + language
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to create request: %v", err))
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Request failed: %v", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to read response: %v", err))
	}

	if resp.StatusCode != http.StatusOK {
		return ErrorResult(fmt.Sprintf("Whisper ASR returned status %d: %s", resp.StatusCode, string(respBody)))
	}

	transcript := string(respBody)
	filename := filepath.Base(filePath)

	return &ToolResult{
		ForLLM:  transcript,
		ForUser: fmt.Sprintf("Transcribed %s (%d chars)", filename, len(transcript)),
	}
}
