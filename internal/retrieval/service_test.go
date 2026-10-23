package retrieval

import (
	"context"
	"strings"
	"testing"

	"subtitle-rag/internal/domain"
)

type mockEmbedClient struct{}

func (m *mockEmbedClient) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	return []float32{1.0, 0.0}, nil
}
func (m *mockEmbedClient) EmbedMany(ctx context.Context, texts []string) ([][]float32, error) {
	return [][]float32{{1.0, 0.0}}, nil
}
func (m *mockEmbedClient) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return []float32{1.0, 0.0}, nil
}
func (m *mockEmbedClient) Dimension() int { return 2 }
func (m *mockEmbedClient) Model() string  { return "mock" }

type mockVectorStore struct {
	hits []domain.SearchHit
}

func (s *mockVectorStore) EnsureCollection(ctx context.Context, dim int) error { return nil }
func (s *mockVectorStore) Upsert(ctx context.Context, chunks []domain.Chunk, vectors [][]float32) error {
	return nil
}
func (s *mockVectorStore) Search(ctx context.Context, vector []float32, limit int, courseID, fileID *string, withVector bool) ([]domain.SearchHit, error) {
	return s.hits, nil
}
func (s *mockVectorStore) GetCollections(ctx context.Context) ([]string, error) {
	return []string{"subs_chunks"}, nil
}

type mockLLM struct {
	answer string
}

func (m *mockLLM) Generate(ctx context.Context, system, user string, temp float64) (string, error) {
	return m.answer, nil
}
func (m *mockLLM) Model() string { return "mock-chat" }

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{1, 0}
	sim := CosineSimilarity(a, b)
	if sim < 0.9999 {
		t.Errorf("expected ~1.0 for identical vectors, got %f", sim)
	}

	c := []float32{0, 1}
	simOrth := CosineSimilarity(a, c)
	if simOrth > 0.0001 {
		t.Errorf("expected 0 for orthogonal vectors, got %f", simOrth)
	}
}

func TestBuildPrompt(t *testing.T) {
	chunks := []domain.SearchHit{
		{
			FileID:  "node-intro",
			StartMs: 1000,
			EndMs:   3000,
			Text:    "Node is a runtime.",
		},
	}

	p := BuildPrompt("What is node?", chunks)
	if !strings.Contains(p.System, "helpful tutor") {
		t.Errorf("system prompt missing tutor instruction")
	}
	if !strings.Contains(p.User, "[node-intro 00:00:01.000–00:00:03.000]") {
		t.Errorf("user context missing formatted citation range: %s", p.User)
	}
	if !strings.Contains(p.User, "Node is a runtime.") {
		t.Errorf("user context missing text: %s", p.User)
	}
}

func TestAnswerQuery_LowConfidence(t *testing.T) {
	store := &mockVectorStore{
		hits: []domain.SearchHit{
			{
				Score:  0.40, // Below default 0.55
				Vector: []float32{0.5, 0.5},
			},
		},
	}
	svc := NewService(&mockEmbedClient{}, store, &mockLLM{answer: "should not be called"})

	resp, err := svc.AnswerQuery(context.Background(), domain.AskRequest{
		Query: "Unknown topic",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Answer != "I don’t know, but I could help in other areas." {
		t.Errorf("expected fallback answer, got %q", resp.Answer)
	}
	if len(resp.Citations) != 0 {
		t.Errorf("expected 0 citations for fallback, got %d", len(resp.Citations))
	}
	if resp.Used.Meta.Reason != "low_confidence" {
		t.Errorf("expected reason 'low_confidence', got %q", resp.Used.Meta.Reason)
	}
}

func TestAnswerQuery_Success(t *testing.T) {
	store := &mockVectorStore{
		hits: []domain.SearchHit{
			{
				CourseID: "nodejs",
				FileID:   "intro",
				StartMs:  1000,
				EndMs:    4000,
				Start:    "00:00:01.000",
				End:      "00:00:04.000",
				Text:     "Node is built on V8.",
				Score:    0.85,
				Vector:   []float32{1.0, 0.0},
			},
		},
	}
	llm := &mockLLM{answer: "Node runs on V8 [intro 00:00:01.000–00:00:04.000]."}
	svc := NewService(&mockEmbedClient{}, store, llm)

	resp, err := svc.AnswerQuery(context.Background(), domain.AskRequest{
		Query: "What engine runs node?",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Answer != "Node runs on V8 [intro 00:00:01.000–00:00:04.000]." {
		t.Errorf("unexpected answer: %q", resp.Answer)
	}
	if len(resp.Citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(resp.Citations))
	}
	if resp.Citations[0].FileID != "intro" || resp.Citations[0].Score != 0.85 {
		t.Errorf("unexpected citation: %+v", resp.Citations[0])
	}
}
