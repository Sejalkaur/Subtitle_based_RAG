package subtitle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"subtitle-rag/internal/domain"
)

func TestChunkCues_Empty(t *testing.T) {
	chunks := ChunkCues(nil, DefaultChunkOptions())
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for nil cues, got %d", len(chunks))
	}
}

func TestChunkCues_ParityWithTestFile(t *testing.T) {
	cues := []domain.Cue{
		{
			CourseID: "test.vtt",
			FileID:   "test.vtt__test",
			Index:    1,
			StartMs:  1000,
			EndMs:    2000,
			Text:     "Hello there!",
		},
		{
			CourseID: "test.vtt",
			FileID:   "test.vtt__test",
			Index:    2,
			StartMs:  2500,
			EndMs:    3500,
			Text:     "General Kenobi.",
		},
	}

	chunks := ChunkCues(cues, DefaultChunkOptions())
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	c := chunks[0]
	if c.ChunkID != "test.vtt__test#1000-61000" {
		t.Errorf("expected chunkId test.vtt__test#1000-61000, got %s", c.ChunkID)
	}
	if c.StartMs != 1000 || c.EndMs != 3500 {
		t.Errorf("expected startMs=1000, endMs=3500, got startMs=%d, endMs=%d", c.StartMs, c.EndMs)
	}
	if c.Text != "Hello there! General Kenobi." {
		t.Errorf("expected text 'Hello there! General Kenobi.', got %q", c.Text)
	}
	if !reflect.DeepEqual(c.LineIndices, []int{1, 2}) {
		t.Errorf("expected lineIndices [1, 2], got %+v", c.LineIndices)
	}
}

func TestChunkCues_ParityWithNodeChunkedFile(t *testing.T) {
	parsedCuesPath := filepath.Join("..", "..", "data", "parsed", "nodejs__01-node-introduction.json")
	expectedChunksPath := filepath.Join("..", "..", "data", "parsed", "chunks_nodejs__01-node-introduction.json")

	cuesBytes, err := os.ReadFile(parsedCuesPath)
	if err != nil {
		t.Skip("node parsed cues file not found, skipping parity test")
		return
	}
	chunksBytes, err := os.ReadFile(expectedChunksPath)
	if err != nil {
		t.Skip("node chunked file not found, skipping parity test")
		return
	}

	var cues []domain.Cue
	if err := json.Unmarshal(cuesBytes, &cues); err != nil {
		t.Fatalf("failed to unmarshal cues: %v", err)
	}

	var expectedChunks []domain.Chunk
	if err := json.Unmarshal(chunksBytes, &expectedChunks); err != nil {
		t.Fatalf("failed to unmarshal expected chunks: %v", err)
	}

	actualChunks := ChunkCues(cues, DefaultChunkOptions())
	if len(actualChunks) != len(expectedChunks) {
		t.Fatalf("chunk count mismatch: expected %d, got %d", len(expectedChunks), len(actualChunks))
	}

	for i := range actualChunks {
		exp := expectedChunks[i]
		act := actualChunks[i]

		if act.ChunkID != exp.ChunkID {
			t.Errorf("chunk[%d].ChunkID mismatch: exp %s, act %s", i, exp.ChunkID, act.ChunkID)
		}
		if act.CourseID != exp.CourseID {
			t.Errorf("chunk[%d].CourseID mismatch: exp %s, act %s", i, exp.CourseID, act.CourseID)
		}
		if act.FileID != exp.FileID {
			t.Errorf("chunk[%d].FileID mismatch: exp %s, act %s", i, exp.FileID, act.FileID)
		}
		if act.StartMs != exp.StartMs {
			t.Errorf("chunk[%d].StartMs mismatch: exp %d, act %d", i, exp.StartMs, act.StartMs)
		}
		if act.EndMs != exp.EndMs {
			t.Errorf("chunk[%d].EndMs mismatch: exp %d, act %d", i, exp.EndMs, act.EndMs)
		}
		if act.Text != exp.Text {
			t.Errorf("chunk[%d].Text mismatch:\nexp: %q\nact: %q", i, exp.Text, act.Text)
		}
		if !reflect.DeepEqual(act.LineIndices, exp.LineIndices) {
			t.Errorf("chunk[%d].LineIndices mismatch: exp %+v, act %+v", i, exp.LineIndices, act.LineIndices)
		}
	}
}
