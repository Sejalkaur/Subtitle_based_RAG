package vectorstore

import (
	"context"

	"subtitle-rag/internal/domain"
)

// Store defines operations required for vector database operations.
type Store interface {
	EnsureCollection(ctx context.Context, dim int) error
	Upsert(ctx context.Context, chunks []domain.Chunk, vectors [][]float32) error
	Search(ctx context.Context, vector []float32, limit int, courseID, fileID *string, withVector bool) ([]domain.SearchHit, error)
	GetCollections(ctx context.Context) ([]string, error)
}
