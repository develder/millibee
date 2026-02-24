package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "standard watch URL",
			input: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			want:  "dQw4w9WgXcQ",
		},
		{
			name:  "short URL",
			input: "https://youtu.be/dQw4w9WgXcQ",
			want:  "dQw4w9WgXcQ",
		},
		{
			name:  "embed URL",
			input: "https://www.youtube.com/embed/dQw4w9WgXcQ",
			want:  "dQw4w9WgXcQ",
		},
		{
			name:  "shorts URL",
			input: "https://www.youtube.com/shorts/dQw4w9WgXcQ",
			want:  "dQw4w9WgXcQ",
		},
		{
			name:  "live URL",
			input: "https://www.youtube.com/live/dQw4w9WgXcQ",
			want:  "dQw4w9WgXcQ",
		},
		{
			name:  "watch URL with extra params",
			input: "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=120",
			want:  "dQw4w9WgXcQ",
		},
		{
			name:  "watch URL without www",
			input: "https://youtube.com/watch?v=dQw4w9WgXcQ",
			want:  "dQw4w9WgXcQ",
		},
		{
			name:    "invalid URL",
			input:   "not-a-url",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "youtube URL without video ID",
			input:   "https://www.youtube.com/watch",
			wantErr: true,
		},
		{
			name:    "youtube URL with empty v param",
			input:   "https://www.youtube.com/watch?v=",
			wantErr: true,
		},
		{
			name:    "non-youtube URL",
			input:   "https://www.example.com/watch?v=dQw4w9WgXcQ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractVideoID(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestYouTubeTranscript_Success_VideoID(t *testing.T) {
	segments := []map[string]any{
		{"text": "Hello world", "start": 0.0, "duration": 2.5},
		{"text": "this is a test", "start": 2.5, "duration": 3.0},
		{"text": "transcript", "start": 5.5, "duration": 1.5},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/transcript", r.URL.Path)
		assert.Equal(t, "dQw4w9WgXcQ", r.URL.Query().Get("video_id"))
		assert.Equal(t, "en", r.URL.Query().Get("language"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(segments)
	}))
	defer server.Close()

	tool := NewYouTubeTranscriptTool(server.URL, 10)
	result := tool.Execute(context.Background(), map[string]any{
		"video_id": "dQw4w9WgXcQ",
	})

	assert.False(t, result.IsError, "expected success, got error: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "Hello world")
	assert.Contains(t, result.ForLLM, "this is a test")
	assert.Contains(t, result.ForLLM, "transcript")
	assert.Contains(t, result.ForUser, "dQw4w9WgXcQ")
	assert.Contains(t, result.ForUser, "chars")
}

func TestYouTubeTranscript_Success_URL(t *testing.T) {
	segments := []map[string]any{
		{"text": "URL extracted", "start": 0.0, "duration": 2.0},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "dQw4w9WgXcQ", r.URL.Query().Get("video_id"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(segments)
	}))
	defer server.Close()

	tool := NewYouTubeTranscriptTool(server.URL, 10)
	result := tool.Execute(context.Background(), map[string]any{
		"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	})

	assert.False(t, result.IsError, "expected success, got error: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "URL extracted")
}

func TestYouTubeTranscript_WithLanguage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "nl", r.URL.Query().Get("language"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]any{
			{"text": "Hallo wereld", "start": 0.0, "duration": 2.0},
		})
	}))
	defer server.Close()

	tool := NewYouTubeTranscriptTool(server.URL, 10)
	result := tool.Execute(context.Background(), map[string]any{
		"video_id": "abc123",
		"language": "nl",
	})

	assert.False(t, result.IsError, "expected success, got error: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "Hallo wereld")
}

func TestYouTubeTranscript_MissingParams(t *testing.T) {
	tool := NewYouTubeTranscriptTool("http://localhost:9999", 10)
	result := tool.Execute(context.Background(), map[string]any{})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "url")
}

func TestYouTubeTranscript_InvalidURL(t *testing.T) {
	tool := NewYouTubeTranscriptTool("http://localhost:9999", 10)
	result := tool.Execute(context.Background(), map[string]any{
		"url": "https://www.example.com/not-youtube",
	})

	assert.True(t, result.IsError)
}

func TestYouTubeTranscript_ServiceUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	tool := NewYouTubeTranscriptTool(server.URL, 10)
	result := tool.Execute(context.Background(), map[string]any{
		"video_id": "dQw4w9WgXcQ",
	})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "500")
}
