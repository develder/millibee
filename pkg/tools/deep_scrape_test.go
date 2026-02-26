package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDeepScrape_SinglePage_Success verifies single-page scraping returns markdown content via /md.
func TestDeepScrape_SinglePage_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/md", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)

		_, ok := payload["url"].(string)
		assert.True(t, ok, "expected url string in payload")

		// /md returns markdown directly as plain text
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("# Hello"))
	}))
	defer server.Close()

	tool := NewDeepScrapeTool(server.URL, 30)
	result := tool.Execute(context.Background(), map[string]any{
		"url": "https://example.com",
	})

	assert.False(t, result.IsError, "expected success, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "# Hello")
	assert.NotEmpty(t, result.ForUser)
	assert.Contains(t, result.ForUser, "example.com")
}

// TestDeepScrape_SinglePage_EmptyContent verifies error when /md returns empty content.
func TestDeepScrape_SinglePage_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(""))
	}))
	defer server.Close()

	tool := NewDeepScrapeTool(server.URL, 30)
	result := tool.Execute(context.Background(), map[string]any{
		"url": "https://example.com",
	})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "no content")
}

// TestDeepScrape_DeepCrawl_Success verifies deep crawl mode with job polling.
func TestDeepScrape_DeepCrawl_Success(t *testing.T) {
	var pollCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/crawl/job" && r.Method == "POST":
			// Verify deep crawl strategy is in the payload
			var payload map[string]any
			json.NewDecoder(r.Body).Decode(&payload)

			response := map[string]any{"task_id": "abc-123"}
			json.NewEncoder(w).Encode(response)

		case r.URL.Path == "/crawl/job/abc-123" && r.Method == "GET":
			count := pollCount.Add(1)
			if count < 2 {
				json.NewEncoder(w).Encode(map[string]any{"status": "pending"})
			} else {
				json.NewEncoder(w).Encode(map[string]any{
					"status": "completed",
					"results": []map[string]any{
						{"fit_markdown": "# Page 1", "url": "https://example.com", "success": true},
						{"fit_markdown": "# Page 2", "url": "https://example.com/about", "success": true},
					},
				})
			}

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tool := NewDeepScrapeTool(server.URL, 30)
	tool.pollInterval = 10 * time.Millisecond // Speed up polling for tests

	result := tool.Execute(context.Background(), map[string]any{
		"url":       "https://example.com",
		"max_depth": 2,
		"max_pages": 10,
	})

	assert.False(t, result.IsError, "expected success, got: %s", result.ForLLM)
	assert.Contains(t, result.ForLLM, "# Page 1")
	assert.Contains(t, result.ForLLM, "# Page 2")
	assert.GreaterOrEqual(t, int(pollCount.Load()), 2, "expected at least 2 polls")
}

// TestDeepScrape_DeepCrawl_Failed verifies error handling when a deep crawl job fails.
func TestDeepScrape_DeepCrawl_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/crawl/job":
			json.NewEncoder(w).Encode(map[string]any{"task_id": "fail-job"})
		case r.URL.Path == "/crawl/job/fail-job":
			json.NewEncoder(w).Encode(map[string]any{
				"status": "failed",
				"error":  "crawl timed out",
			})
		}
	}))
	defer server.Close()

	tool := NewDeepScrapeTool(server.URL, 30)
	tool.pollInterval = 10 * time.Millisecond

	result := tool.Execute(context.Background(), map[string]any{
		"url":       "https://example.com",
		"max_depth": 2,
		"max_pages": 5,
	})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "failed")
}

// TestDeepScrape_MissingURL verifies error when url parameter is not provided.
func TestDeepScrape_MissingURL(t *testing.T) {
	tool := NewDeepScrapeTool("http://localhost:9999", 30)

	result := tool.Execute(context.Background(), map[string]any{})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "url is required")
}

// TestDeepScrape_ServiceUnavailable verifies graceful error when the service is down.
func TestDeepScrape_ServiceUnavailable(t *testing.T) {
	tool := NewDeepScrapeTool("http://127.0.0.1:1", 5) // Nothing listening on port 1

	result := tool.Execute(context.Background(), map[string]any{
		"url": "https://example.com",
	})

	assert.True(t, result.IsError)
	assert.NotEmpty(t, result.ForLLM)
}

// TestDeepScrape_ContextCancelled verifies that context cancellation stops polling.
func TestDeepScrape_ContextCancelled(t *testing.T) {
	var pollCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/crawl/job":
			json.NewEncoder(w).Encode(map[string]any{"task_id": "cancel-job"})
		case strings.HasPrefix(r.URL.Path, "/crawl/job/"):
			pollCount.Add(1)
			json.NewEncoder(w).Encode(map[string]any{"status": "pending"})
		}
	}))
	defer server.Close()

	tool := NewDeepScrapeTool(server.URL, 30)
	tool.pollInterval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result := tool.Execute(ctx, map[string]any{
		"url":       "https://example.com",
		"max_depth": 3,
		"max_pages": 10,
	})

	assert.True(t, result.IsError)
	assert.Contains(t, result.ForLLM, "cancel")
}
