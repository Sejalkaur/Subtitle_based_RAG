package config

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration values.
type Config struct {
	Port               int
	NodeEnv            string
	LogLevel           string
	QdrantURL          string
	QdrantAPIKey       string
	QdrantCollection   string
	EmbeddingsProvider string
	GeminiAPIKey       string
	EmbeddingsModel    string
	EmbeddingsDim      int
	ChatModel          string
	ChunkIDNamespace   string
}

// Load loads configuration from .env and environment variables.
func Load() (*Config, error) {
	// Try loading .env from current directory or server/.env
	if err := godotenv.Load(); err != nil {
		if err2 := godotenv.Load("server/.env"); err2 != nil {
			slog.Debug("No .env file found; using environment variables directly")
		}
	}

	port := 3001
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			port = p
		}
	}

	dim := 768
	if dimStr := os.Getenv("EMBEDDINGS_DIM"); dimStr != "" {
		if d, err := strconv.Atoi(dimStr); err == nil && d > 0 {
			dim = d
		}
	}

	cfg := &Config{
		Port:               port,
		NodeEnv:            getEnv("NODE_ENV", "development"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		QdrantURL:          getEnv("QDRANT_URL", "http://localhost:6333"),
		QdrantAPIKey:       os.Getenv("QDRANT_API_KEY"),
		QdrantCollection:   getEnv("QDRANT_COLLECTION", "subs_chunks"),
		EmbeddingsProvider: getEnv("EMBEDDINGS_PROVIDER", "gemini"),
		GeminiAPIKey:       os.Getenv("GEMINI_API_KEY"),
		EmbeddingsModel:    getEnv("EMBEDDINGS_MODEL", "gemini-embedding-2"),
		EmbeddingsDim:      dim,
		ChatModel:          getEnv("CHAT_MODEL", "gemini-3.8-flash"),
		ChunkIDNamespace:   getEnv("CHUNK_ID_NAMESPACE", "6ba7b811-9dad-11d1-80b4-00c04fd430c8"),
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}
