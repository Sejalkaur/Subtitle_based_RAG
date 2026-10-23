package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestGeminiClient_EmbedOne(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		if req.TaskType != "RETRIEVAL_DOCUMENT" {
			t.Errorf("expected taskType RETRIEVAL_DOCUMENT, got %s", req.TaskType)
		}
		if req.OutputDimensionality != 768 {
			t.Errorf("expected dim 768, got %d", req.OutputDimensionality)
		}

		resp := embedResponse{}
		resp.Embedding.Values = make([]float32, 768)
		resp.Embedding.Values[0] = 0.42
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, err := NewGeminiClient(GeminiOptions{
		APIKey:  "test-api-key",
		Model:   "gemini-embedding-2",
		Dim:     768,
		BaseURL: ts.URL,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	vec, err := client.EmbedOne(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("EmbedOne failed: %v", err)
	}

	if len(vec) != 768 || vec[0] != 0.42 {
		t.Errorf("unexpected vector: length %d, first val %f", len(vec), vec[0])
	}
}

func TestGeminiClient_EmbedMany(t *testing.T) {
	var count int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		resp := embedResponse{}
		resp.Embedding.Values = []float32{0.1, 0.2, 0.3}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, err := NewGeminiClient(GeminiOptions{
		APIKey:  "test-api-key",
		BaseURL: ts.URL,
		Dim:     3,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	texts := []string{"t1", "t2", "t3", "t4", "t5"}
	vecs, err := client.EmbedMany(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedMany failed: %v", err)
	}

	if len(vecs) != 5 {
		t.Fatalf("expected 5 vectors, got %d", len(vecs))
	}
	if atomic.LoadInt32(&count) != 5 {
		t.Errorf("expected 5 HTTP calls, got %d", count)
	}
}

func TestGeminiClient_EmbedQuery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embedRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.TaskType != "RETRIEVAL_QUERY" {
			t.Errorf("expected taskType RETRIEVAL_QUERY, got %s", req.TaskType)
		}
		resp := embedResponse{}
		resp.Embedding.Values = []float32{0.99}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client, _ := NewGeminiClient(GeminiOptions{
		APIKey:  "test-api-key",
		BaseURL: ts.URL,
	})

	vec, err := client.EmbedQuery(context.Background(), "search query")
	if err != nil {
		t.Fatalf("EmbedQuery failed: %v", err)
	}
	if len(vec) != 1 || vec[0] != 0.99 {
		t.Errorf("unexpected query vector: %+v", vec)
	}
}
