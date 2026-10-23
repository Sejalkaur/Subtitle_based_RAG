package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"subtitle-rag/internal/config"
	"subtitle-rag/internal/embedding"
	"subtitle-rag/internal/httpapi"
	"subtitle-rag/internal/observability"
	"subtitle-rag/internal/retrieval"
	"subtitle-rag/internal/vectorstore"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := observability.InitLogger(cfg.LogLevel, cfg.NodeEnv)

	// Initialize Gemini Embedding client
	embedder, err := embedding.NewGeminiClient(embedding.GeminiOptions{
		APIKey: cfg.GeminiAPIKey,
		Model:  cfg.EmbeddingsModel,
		Dim:    cfg.EmbeddingsDim,
	})
	if err != nil {
		logger.Warn("Gemini embedder initialization warning", slog.Any("error", err))
	}

	// Initialize Qdrant Vector Store client
	store, err := vectorstore.NewQdrantClient(vectorstore.QdrantOptions{
		URL:           cfg.QdrantURL,
		APIKey:        cfg.QdrantAPIKey,
		Collection:    cfg.QdrantCollection,
		NamespaceUUID: cfg.ChunkIDNamespace,
	})
	if err != nil {
		logger.Error("Failed to initialize Qdrant client", slog.Any("error", err))
		os.Exit(1)
	}

	// Initialize Gemini LLM client
	var llmClient retrieval.LLMClient
	if cfg.GeminiAPIKey != "" {
		llmClient, err = retrieval.NewGeminiLLMClient(retrieval.GeminiLLMOptions{
			APIKey: cfg.GeminiAPIKey,
			Model:  cfg.ChatModel,
		})
		if err != nil {
			logger.Warn("Gemini LLM initialization warning", slog.Any("error", err))
		}
	}

	// Initialize Retrieval and API services
	retrievalSvc := retrieval.NewService(embedder, store, llmClient)
	handler := httpapi.NewAPIHandler(cfg, retrievalSvc)
	router := httpapi.NewRouter(handler)

	server := &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("==========================================")
		logger.Info(fmt.Sprintf("🚀 Go API running on http://localhost:%d", cfg.Port))
		logger.Info(fmt.Sprintf("🤖 Chat Model: %s", cfg.ChatModel))
		logger.Info(fmt.Sprintf("📐 Embeddings: %s (%s)", cfg.EmbeddingsModel, cfg.EmbeddingsProvider))
		logger.Info(fmt.Sprintf("📦 Qdrant DB:  %s [Collection: %s]", cfg.QdrantURL, cfg.QdrantCollection))
		logger.Info("==========================================")

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server failed to listen", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Non-blocking background check for Qdrant connectivity
	go func() {
		checkCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cols, err := store.GetCollections(checkCtx)
		if err != nil {
			logger.Warn(fmt.Sprintf("⚠️  Cannot reach Qdrant at %s. Make sure Docker is running: run \"docker compose up -d\"", cfg.QdrantURL), slog.Any("error", err))
		} else {
			logger.Info("✅ Successfully connected to Qdrant", slog.Any("collections", cols))
		}
	}()

	<-ctx.Done()
	logger.Info("Gracefully shutting down server...")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced shutdown", slog.Any("error", err))
	}
	logger.Info("Server stopped cleanly")
}
