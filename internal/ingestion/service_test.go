package ingestion

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"subtitle-rag/internal/domain"
	"subtitle-rag/internal/subtitle"
)

type mockEmbedder struct{}

func (m *mockEmbedder) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}
func (m *mockEmbedder) EmbedMany(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2}
	}
	return out, nil
}
func (m *mockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}
func (m *mockEmbedder) Dimension() int { return 2 }
func (m *mockEmbedder) Model() string  { return "mock" }

type mockStore struct {
	upsertedChunks  []domain.Chunk
	upsertedVectors [][]float32
}

func (s *mockStore) EnsureCollection(ctx context.Context, dim int) error { return nil }
func (s *mockStore) Upsert(ctx context.Context, chunks []domain.Chunk, vectors [][]float32) error {
	s.upsertedChunks = append(s.upsertedChunks, chunks...)
	s.upsertedVectors = append(s.upsertedVectors, vectors...)
	return nil
}
func (s *mockStore) Search(ctx context.Context, vector []float32, limit int, courseID, fileID *string, withVector bool) ([]domain.SearchHit, error) {
	return nil, nil
}
func (s *mockStore) GetCollections(ctx context.Context) ([]string, error) {
	return []string{"subs_chunks"}, nil
}

func TestIngestionPipeline_EndToEnd(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rag-ingest-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	rawDir := filepath.Join(tmpDir, "data", "raw", "nodejs")
	if err := os.MkdirAll(rawDir, 0755); err != nil {
		t.Fatalf("failed to create raw dir: %v", err)
	}

	testVTT := `WEBVTT

00:00:01.000 --> 00:00:02.000
Hello from test VTT.

00:00:03.000 --> 00:00:04.000
Second cue in nodejs.
`
	vttFile := filepath.Join(rawDir, "test-course.vtt")
	if err := os.WriteFile(vttFile, []byte(testVTT), 0644); err != nil {
		t.Fatalf("failed to write test vtt: %v", err)
	}

	parsedDir := filepath.Join(tmpDir, "data", "parsed")
	store := &mockStore{}
	embedder := &mockEmbedder{}
	svc := NewIngestionService(embedder, store)

	ctx := context.Background()

	// 1. Ingest
	ingestStats, err := svc.IngestVTTFiles(ctx, []string{filepath.Join(rawDir, "*.vtt")}, parsedDir)
	if err != nil {
		t.Fatalf("IngestVTTFiles failed: %v", err)
	}
	if ingestStats.SuccessFiles != 1 || ingestStats.TotalCues != 2 {
		t.Errorf("unexpected ingest stats: %+v", ingestStats)
	}

	// 2. Chunk
	chunkStats, err := svc.ChunkParsedFiles(ctx, []string{filepath.Join(parsedDir, "*.json")}, subtitle.DefaultChunkOptions())
	if err != nil {
		t.Fatalf("ChunkParsedFiles failed: %v", err)
	}
	if chunkStats.SuccessFiles != 1 || chunkStats.TotalChunks != 1 {
		t.Errorf("unexpected chunk stats: %+v", chunkStats)
	}

	// 3. Embed & Upsert
	embedStats, err := svc.EmbedAndUpsert(ctx, []string{filepath.Join(parsedDir, "chunks_*.json")}, 10)
	if err != nil {
		t.Fatalf("EmbedAndUpsert failed: %v", err)
	}
	if embedStats.Upserted != 1 {
		t.Errorf("unexpected embed stats: %+v", embedStats)
	}
	if len(store.upsertedChunks) != 1 {
		t.Fatalf("expected 1 chunk upserted to store, got %d", len(store.upsertedChunks))
	}
	if store.upsertedChunks[0].CourseID != "nodejs" {
		t.Errorf("expected courseId 'nodejs', got %s", store.upsertedChunks[0].CourseID)
	}
}
