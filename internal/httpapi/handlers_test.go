package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"subtitle-rag/internal/config"
	"subtitle-rag/internal/domain"
	"subtitle-rag/internal/retrieval"
)

type mockEmbedder struct{}

func (m *mockEmbedder) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	return []float32{1.0, 0.0}, nil
}
func (m *mockEmbedder) EmbedMany(ctx context.Context, texts []string) ([][]float32, error) {
	return [][]float32{{1.0, 0.0}}, nil
}
func (m *mockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return []float32{1.0, 0.0}, nil
}
func (m *mockEmbedder) Dimension() int { return 2 }
func (m *mockEmbedder) Model() string  { return "mock" }

type mockVectorStore struct{}

func (s *mockVectorStore) EnsureCollection(ctx context.Context, dim int) error { return nil }
func (s *mockVectorStore) Upsert(ctx context.Context, chunks []domain.Chunk, vectors [][]float32) error {
	return nil
}
func (s *mockVectorStore) Search(ctx context.Context, vector []float32, limit int, courseID, fileID *string, withVector bool) ([]domain.SearchHit, error) {
	return []domain.SearchHit{
		{
			CourseID: "nodejs",
			FileID:   "intro",
			Start:    "00:00:01.000",
			End:      "00:00:04.000",
			StartMs:  1000,
			EndMs:    4000,
			Text:     "Node is built on V8.",
			Score:    0.92,
			Vector:   []float32{1.0, 0.0},
		},
	}, nil
}
func (s *mockVectorStore) GetCollections(ctx context.Context) ([]string, error) {
	return []string{"subs_chunks"}, nil
}

type mockLLM struct{}

func (m *mockLLM) Generate(ctx context.Context, system, user string, temp float64) (string, error) {
	return "Mock answer with citation [intro 00:00:01.000–00:00:04.000].", nil
}
func (m *mockLLM) Model() string { return "gemini-3.8-flash" }

func setupTestRouter() http.Handler {
	cfg := &config.Config{
		ChatModel:       "gemini-3.8-flash",
		EmbeddingsModel: "gemini-embedding-2",
	}
	svc := retrieval.NewService(&mockEmbedder{}, &mockVectorStore{}, &mockLLM{})
	handler := NewAPIHandler(cfg, svc)
	return NewRouter(handler)
}

func TestHealthEndpoint(t *testing.T) {
	router := setupTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal health response: %v", err)
	}

	if body["ok"] != true || body["service"] != "subtitle-rag-api" || body["chatModel"] != "gemini-3.8-flash" {
		t.Errorf("unexpected health response body: %+v", body)
	}
}

func TestRootEndpoint(t *testing.T) {
	router := setupTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["ok"] != true {
		t.Errorf("unexpected root response body: %+v", body)
	}
}

func TestAskEndpoint_Validation(t *testing.T) {
	router := setupTestRouter()

	// Missing query
	reqBody := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ask", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty query, got %d", rec.Code)
	}
}

func TestAskEndpoint_Success(t *testing.T) {
	router := setupTestRouter()

	reqBody := []byte(`{"query": "What is Node.js?"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ask", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domain.AskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode AskResponse: %v", err)
	}

	if resp.Answer != "Mock answer with citation [intro 00:00:01.000–00:00:04.000]." {
		t.Errorf("unexpected answer: %q", resp.Answer)
	}
	if len(resp.Citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(resp.Citations))
	}
	if resp.Citations[0].CourseID != "nodejs" || resp.Citations[0].Score != 0.92 {
		t.Errorf("unexpected citation: %+v", resp.Citations[0])
	}
	if resp.Used.Query != "What is Node.js?" || resp.Used.TopK != 80 {
		t.Errorf("unexpected used metadata: %+v", resp.Used)
	}
}
