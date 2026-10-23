package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"subtitle-rag/internal/config"
	"subtitle-rag/internal/embedding"
	"subtitle-rag/internal/ingestion"
	"subtitle-rag/internal/observability"
	"subtitle-rag/internal/subtitle"
	"subtitle-rag/internal/vectorstore"
)

func main() {
	mode := flag.String("mode", "all", "Ingestion step: 'parse', 'chunk', 'embed', or 'all'")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := observability.InitLogger(cfg.LogLevel, cfg.NodeEnv)

	embedder, err := embedding.NewGeminiClient(embedding.GeminiOptions{
		APIKey: cfg.GeminiAPIKey,
		Model:  cfg.EmbeddingsModel,
		Dim:    cfg.EmbeddingsDim,
	})
	if err != nil && (*mode == "embed" || *mode == "all") {
		logger.Error("Gemini client initialization failed", slog.Any("error", err))
		os.Exit(1)
	}

	store, err := vectorstore.NewQdrantClient(vectorstore.QdrantOptions{
		URL:           cfg.QdrantURL,
		APIKey:        cfg.QdrantAPIKey,
		Collection:    cfg.QdrantCollection,
		NamespaceUUID: cfg.ChunkIDNamespace,
	})
	if err != nil && (*mode == "embed" || *mode == "all") {
		logger.Error("Qdrant client initialization failed", slog.Any("error", err))
		os.Exit(1)
	}

	svc := ingestion.NewIngestionService(embedder, store)
	ctx := context.Background()

	rawPatterns := []string{
		"data/raw/**/*.vtt",
		"data/raw/*.vtt",
		"../../data/raw/**/*.vtt",
		"../../data/raw/*.vtt",
	}
	parsedDir := "data/parsed"

	if *mode == "parse" || *mode == "all" {
		logger.Info("Starting VTT subtitle ingestion...")
		stats, err := svc.IngestVTTFiles(ctx, rawPatterns, parsedDir)
		if err != nil {
			logger.Error("Ingest failed", slog.Any("error", err))
		} else {
			logger.Info("Ingest complete", slog.Int("files", stats.SuccessFiles), slog.Int("cues", stats.TotalCues))
		}
	}

	if *mode == "chunk" || *mode == "all" {
		logger.Info("Starting subtitle chunking...")
		chunkPatterns := []string{
			"data/parsed/*.json",
			"../../data/parsed/*.json",
		}
		stats, err := svc.ChunkParsedFiles(ctx, chunkPatterns, subtitle.DefaultChunkOptions())
		if err != nil {
			logger.Error("Chunking failed", slog.Any("error", err))
		} else {
			logger.Info("Chunking complete", slog.Int("files", stats.SuccessFiles), slog.Int("chunks", stats.TotalChunks))
		}
	}

	if *mode == "embed" || *mode == "all" {
		logger.Info("Starting embedding generation & Qdrant upsert...")
		embedPatterns := []string{
			"data/parsed/chunks_*.json",
			"../../data/parsed/chunks_*.json",
		}
		stats, err := svc.EmbedAndUpsert(ctx, embedPatterns, 32)
		if err != nil {
			logger.Error("Embed & Upsert failed", slog.Any("error", err))
		} else {
			logger.Info("Embedding & Upsert complete", slog.Int("upserted", stats.Upserted))
		}
	}
}
