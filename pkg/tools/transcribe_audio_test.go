package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTranscribeAudio_Success verifies successful transcription of an audio file.
func TestTranscribeAudio_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/asr", r.URL.Path)
		assert.Equal(t, "txt", r.URL.Query().Get("output"))

		err := r.ParseMultipartForm(10 << 20)
		require.NoError(t, err)

		file, header, err := r.FormFile("audio_file")
		require.NoError(t, err)
		defer file.Close()

		assert.NotEmpty(t, header.Filename)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello world transcript"))
	}))
	defer server.Close()

	// Create temp audio file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.wav")
	err := os.WriteFile(tmpFile, []byte("fake audio content"), 0644)
	require.NoError(t, err)

	tool := NewTranscribeAudioTool(server.URL, 30)
	ctx := context.Background()
	result := tool.Execute(ctx, map[string]any{
		"file_path": tmpFile,
	})

	assert.False(t, result.IsError, "Expected success, got error: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "Hello world transcript")
	assert.Contains(t, result.ForUser, "Transcribed")
	assert.Contains(t, result.ForUser, "test.wav")
	assert.Contains(t, result.ForUser, "22 chars")
}

// TestTranscribeAudio_WithLanguage verifies language parameter is passed to the API.
func TestTranscribeAudio_WithLanguage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "nl", r.URL.Query().Get("language"))
		assert.Equal(t, "txt", r.URL.Query().Get("output"))

		err := r.ParseMultipartForm(10 << 20)
		require.NoError(t, err)
		_, _, err = r.FormFile("audio_file")
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Nederlands transcript"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.wav")
	os.WriteFile(tmpFile, []byte("fake audio"), 0644)

	tool := NewTranscribeAudioTool(server.URL, 30)
	result := tool.Execute(context.Background(), map[string]any{
		"file_path": tmpFile,
		"language":  "nl",
	})

	assert.False(t, result.IsError)
	assert.Contains(t, result.ForLLM, "Nederlands transcript")
}

// TestTranscribeAudio_OutputFormats verifies output format parameter is passed correctly.
func TestTranscribeAudio_OutputFormats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "srt", r.URL.Query().Get("output"))

		err := r.ParseMultipartForm(10 << 20)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("1\n00:00:00,000 --> 00:00:05,000\nHello world\n"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.wav")
	os.WriteFile(tmpFile, []byte("fake audio"), 0644)

	tool := NewTranscribeAudioTool(server.URL, 30)
	result := tool.Execute(context.Background(), map[string]any{
		"file_path": tmpFile,
		"output":    "srt",
	})

	assert.False(t, result.IsError)
	assert.Contains(t, result.ForLLM, "Hello world")
}

// TestTranscribeAudio_YouTubeURL_Rejected verifies YouTube URLs are rejected with helpful message.
func TestTranscribeAudio_YouTubeURL_Rejected(t *testing.T) {
	tool := NewTranscribeAudioTool("http://localhost:9999", 30)

	tests := []struct {
		name string
		url  string
	}{
		{"youtube.com", "https://www.youtube.com/watch?v=abc"},
		{"youtu.be", "https://youtu.be/abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.Execute(context.Background(), map[string]any{
				"file_path": tt.url,
			})

			assert.True(t, result.IsError)
			assert.Contains(t, result.ForLLM, "youtube_transcript")
			assert.Contains(t, result.ForLLM, "yt-dlp")
		})
	}
}

// TestTranscribeAudio_FileNotFound verifies error for non-existent file.
func TestTranscribeAudio_FileNotFound(t *testing.T) {
	tool := NewTranscribeAudioTool("http://localhost:9999", 30)
	result := tool.Execute(context.Background(), map[string]any{
		"file_path": "/nonexistent/path/audio.wav",
	})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "open")
}

// TestTranscribeAudio_MissingFilePath verifies error when file_path is not provided.
func TestTranscribeAudio_MissingFilePath(t *testing.T) {
	tool := NewTranscribeAudioTool("http://localhost:9999", 30)
	result := tool.Execute(context.Background(), map[string]any{})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "file_path")
}

// TestTranscribeAudio_ServiceUnavailable verifies graceful error handling for server errors.
func TestTranscribeAudio_ServiceUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.wav")
	os.WriteFile(tmpFile, []byte("fake audio"), 0644)

	tool := NewTranscribeAudioTool(server.URL, 30)
	result := tool.Execute(context.Background(), map[string]any{
		"file_path": tmpFile,
	})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "500")
}

// TestTranscribeAudio_DefaultTimeout verifies default timeout is set when <= 0.
func TestTranscribeAudio_DefaultTimeout(t *testing.T) {
	tool := NewTranscribeAudioTool("http://localhost:9999", 0)
	assert.NotNil(t, tool)
	assert.NotNil(t, tool.client)
}

// TestTranscribeAudio_ToolMetadata verifies Name, Description, and Parameters.
func TestTranscribeAudio_ToolMetadata(t *testing.T) {
	tool := NewTranscribeAudioTool("http://localhost:9999", 30)

	assert.Equal(t, "transcribe_audio", tool.Name())
	assert.Contains(t, tool.Description(), "Whisper")
	assert.Contains(t, tool.Description(), "speech-to-text")

	params := tool.Parameters()
	props, ok := params["properties"].(map[string]any)
	assert.True(t, ok)
	assert.Contains(t, props, "file_path")
	assert.Contains(t, props, "language")
	assert.Contains(t, props, "output")

	required, ok := params["required"].([]string)
	assert.True(t, ok)
	assert.Contains(t, required, "file_path")
}
