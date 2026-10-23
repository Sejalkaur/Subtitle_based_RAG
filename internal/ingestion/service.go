package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"subtitle-rag/internal/domain"
	"subtitle-rag/internal/embedding"
	"subtitle-rag/internal/subtitle"
	"subtitle-rag/internal/vectorstore"
)

type IngestStats struct {
	TotalFiles   int
	SuccessFiles int
	FailedFiles  int
	TotalCues    int
}

type ChunkStats struct {
	TotalFiles   int
	SuccessFiles int
	FailedFiles  int
	TotalChunks  int
}

type EmbedStats struct {
	TotalFiles  int
	TotalChunks int
	Upserted    int
}

// IngestionService orchestrates parsing, chunking, and vector ingestion.
type IngestionService struct {
	embedder embedding.Client
	store    vectorstore.Store
}

func NewIngestionService(embedder embedding.Client, store vectorstore.Store) *IngestionService {
	return &IngestionService{
		embedder: embedder,
		store:    store,
	}
}

// IngestVTTFiles finds and parses raw .vtt files into normalized JSON cues.
func (s *IngestionService) IngestVTTFiles(ctx context.Context, patterns []string, outDir string) (IngestStats, error) {
	var stats IngestStats
	files, err := resolveFilePatterns(patterns)
	if err != nil {
		return stats, err
	}

	if len(files) == 0 {
		return stats, fmt.Errorf("no .vtt files found matching patterns")
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return stats, fmt.Errorf("create output directory: %w", err)
	}

	stats.TotalFiles = len(files)
	for _, absPath := range files {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}

		courseID := subtitle.DeriveCourseID(absPath)
		base := filepath.Base(absPath)
		fileBase := strings.TrimSuffix(base, filepath.Ext(base))
		fileID := fmt.Sprintf("%s__%s", courseID, fileBase)

		content, err := os.ReadFile(absPath)
		if err != nil {
			slog.Error("[fail] read vtt file", slog.String("path", absPath), slog.Any("error", err))
			stats.FailedFiles++
			continue
		}

		cues := subtitle.ParseVTT(string(content), fileID, courseID)
		outPath := filepath.Join(outDir, fmt.Sprintf("%s.json", fileID))

		jsonBytes, err := json.MarshalIndent(cues, "", "  ")
		if err != nil {
			slog.Error("[fail] marshal cues", slog.String("path", absPath), slog.Any("error", err))
			stats.FailedFiles++
			continue
		}

		if err := os.WriteFile(outPath, jsonBytes, 0644); err != nil {
			slog.Error("[fail] write parsed cues", slog.String("path", outPath), slog.Any("error", err))
			stats.FailedFiles++
			continue
		}

		slog.Info("[ok] ingested vtt",
			slog.String("source", filepath.Base(absPath)),
			slog.String("output", filepath.Base(outPath)),
			slog.Int("cues", len(cues)),
		)
		stats.SuccessFiles++
		stats.TotalCues += len(cues)
	}

	return stats, nil
}

// ChunkParsedFiles groups normalized cues into time windows and saves chunks_*.json.
func (s *IngestionService) ChunkParsedFiles(ctx context.Context, patterns []string, opts subtitle.ChunkOptions) (ChunkStats, error) {
	var stats ChunkStats
	files, err := resolveFilePatterns(patterns)
	if err != nil {
		return stats, err
	}

	// Filter out already generated chunks_*.json
	var validFiles []string
	for _, f := range files {
		if !strings.HasPrefix(filepath.Base(f), "chunks_") {
			validFiles = append(validFiles, f)
		}
	}

	if len(validFiles) == 0 {
		return stats, fmt.Errorf("no parsed cue files found")
	}

	stats.TotalFiles = len(validFiles)
	for _, absPath := range validFiles {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			slog.Error("[fail] read cue file", slog.String("path", absPath), slog.Any("error", err))
			stats.FailedFiles++
			continue
		}

		var cues []domain.Cue
		if err := json.Unmarshal(content, &cues); err != nil {
			slog.Error("[fail] unmarshal cues", slog.String("path", absPath), slog.Any("error", err))
			stats.FailedFiles++
			continue
		}

		chunks := subtitle.ChunkCues(cues, opts)
		dir := filepath.Dir(absPath)
		base := filepath.Base(absPath)
		outPath := filepath.Join(dir, fmt.Sprintf("chunks_%s", base))

		jsonBytes, err := json.MarshalIndent(chunks, "", "  ")
		if err != nil {
			slog.Error("[fail] marshal chunks", slog.String("path", absPath), slog.Any("error", err))
			stats.FailedFiles++
			continue
		}

		if err := os.WriteFile(outPath, jsonBytes, 0644); err != nil {
			slog.Error("[fail] write chunks", slog.String("path", outPath), slog.Any("error", err))
			stats.FailedFiles++
			continue
		}

		slog.Info("[ok] chunked file",
			slog.String("source", base),
			slog.String("output", filepath.Base(outPath)),
			slog.Int("chunks", len(chunks)),
		)
		stats.SuccessFiles++
		stats.TotalChunks += len(chunks)
	}

	return stats, nil
}

// EmbedAndUpsert loads chunk JSON files, generates embeddings in batches, and upserts to Qdrant.
func (s *IngestionService) EmbedAndUpsert(ctx context.Context, patterns []string, batchSize int) (EmbedStats, error) {
	var stats EmbedStats
	if batchSize <= 0 {
		batchSize = 32
	}

	files, err := resolveFilePatterns(patterns)
	if err != nil {
		return stats, err
	}

	var chunkFiles []string
	for _, f := range files {
		if strings.HasPrefix(filepath.Base(f), "chunks_") {
			chunkFiles = append(chunkFiles, f)
		}
	}
	if len(chunkFiles) == 0 {
		return stats, fmt.Errorf("no chunks_*.json files found")
	}

	stats.TotalFiles = len(chunkFiles)
	if err := s.store.EnsureCollection(ctx, s.embedder.Dimension()); err != nil {
		return stats, fmt.Errorf("ensure collection: %w", err)
	}

	var allChunks []domain.Chunk
	for _, f := range chunkFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			slog.Error("read chunk file failed", slog.String("file", f), slog.Any("error", err))
			continue
		}
		var chunks []domain.Chunk
		if err := json.Unmarshal(content, &chunks); err != nil {
			slog.Error("unmarshal chunk file failed", slog.String("file", f), slog.Any("error", err))
			continue
		}
		for i := range chunks {
			chunks[i].SourceFile = filepath.Base(f)
			allChunks = append(allChunks, chunks[i])
		}
	}

	stats.TotalChunks = len(allChunks)
	slog.Info("[embed] total chunks loaded", slog.Int("count", len(allChunks)))

	for i := 0; i < len(allChunks); i += batchSize {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}

		end := i + batchSize
		if end > len(allChunks) {
			end = len(allChunks)
		}

		batch := allChunks[i:end]
		texts := make([]string, len(batch))
		for j, c := range batch {
			texts[j] = c.Text
		}

		slog.Info(fmt.Sprintf("[embed] batching: %d texts...", len(batch)))
		vectors, err := s.embedder.EmbedMany(ctx, texts)
		if err != nil {
			return stats, fmt.Errorf("embed batch [%d:%d]: %w", i, end, err)
		}

		if err := s.store.Upsert(ctx, batch, vectors); err != nil {
			return stats, fmt.Errorf("upsert batch [%d:%d]: %w", i, end, err)
		}

		stats.Upserted += len(batch)
		slog.Info(fmt.Sprintf("[upsert] %d/%d", stats.Upserted, len(allChunks)))
	}

	return stats, nil
}

// resolveFilePatterns finds all files matching glob patterns or recursive walks.
func resolveFilePatterns(patterns []string) ([]string, error) {
	var fileList []string
	seen := make(map[string]bool)

	for _, pat := range patterns {
		// If pattern contains "**", handle directory walking
		if strings.Contains(pat, "**") {
			prefix := pat[:strings.Index(pat, "**")]
			ext := filepath.Ext(pat)
			if prefix == "" {
				prefix = "."
			}
			_ = filepath.Walk(prefix, func(path string, info os.FileInfo, err error) error {
				if err != nil || info == nil || info.IsDir() {
					return nil
				}
				if ext == "" || strings.EqualFold(filepath.Ext(path), ext) {
					abs, _ := filepath.Abs(path)
					if !seen[abs] {
						seen[abs] = true
						fileList = append(fileList, abs)
					}
				}
				return nil
			})
			continue
		}

		matches, err := filepath.Glob(pat)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err == nil && !info.IsDir() {
				abs, _ := filepath.Abs(m)
				if !seen[abs] {
					seen[abs] = true
					fileList = append(fileList, abs)
				}
			}
		}
	}

	sort.Strings(fileList)
	return fileList, nil
}
