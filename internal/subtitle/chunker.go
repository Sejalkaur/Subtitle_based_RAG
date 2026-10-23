package subtitle

import (
	"fmt"
	"sort"
	"strings"

	"subtitle-rag/internal/domain"
)

// ChunkOptions configures the sliding window chunker.
type ChunkOptions struct {
	WindowMs  int64
	OverlapMs int64
}

// DefaultChunkOptions returns the standard 60s window with 10s overlap.
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{
		WindowMs:  60000,
		OverlapMs: 10000,
	}
}

// FormatChunkID creates the standard chunkId format: "${fileId}#${startMs}-${endMs}".
func FormatChunkID(fileID string, startMs, endMs int64) string {
	return fmt.Sprintf("%s#%d-%d", fileID, startMs, endMs)
}

// ChunkCues groups normalized cues into time windows with overlap, reproducing Node.js chunkCues.
func ChunkCues(cues []domain.Cue, opts ChunkOptions) []domain.Chunk {
	if len(cues) == 0 {
		return nil
	}

	windowMs := opts.WindowMs
	if windowMs <= 0 {
		windowMs = 60000
	}
	overlapMs := opts.OverlapMs
	if overlapMs < 0 {
		overlapMs = 10000
	}

	// Make a copy and sort by startMs
	sorted := make([]domain.Cue, len(cues))
	copy(sorted, cues)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].StartMs < sorted[j].StartMs
	})

	var chunks []domain.Chunk
	winStart := sorted[0].StartMs
	lastEndMs := sorted[len(sorted)-1].EndMs
	slideStep := windowMs - overlapMs
	if slideStep <= 0 {
		slideStep = windowMs
	}

	for winStart <= lastEndMs {
		winEnd := winStart + windowMs

		// Select cues inside this window: c.startMs < winEnd && c.endMs > winStart
		var inside []domain.Cue
		for _, c := range sorted {
			if c.StartMs < winEnd && c.EndMs > winStart {
				inside = append(inside, c)
			}
		}

		if len(inside) > 0 {
			texts := make([]string, len(inside))
			indices := make([]int, len(inside))
			for i, c := range inside {
				texts[i] = c.Text
				indices[i] = c.Index
			}

			chunk := domain.Chunk{
				ChunkID:     FormatChunkID(sorted[0].FileID, winStart, winEnd),
				CourseID:    inside[0].CourseID,
				FileID:      sorted[0].FileID,
				StartMs:     inside[0].StartMs,
				EndMs:       inside[len(inside)-1].EndMs,
				Text:        strings.Join(texts, " "),
				LineIndices: indices,
			}
			chunks = append(chunks, chunk)
		}

		winStart += slideStep
	}

	return chunks
}
