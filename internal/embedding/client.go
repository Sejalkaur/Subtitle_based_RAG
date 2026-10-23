package embedding

import "context"

// Client defines the contract for embedding generation.
type Client interface {
	EmbedOne(ctx context.Context, text string) ([]float32, error)
	EmbedMany(ctx context.Context, texts []string) ([][]float32, error)
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
	Dimension() int
	Model() string
}
