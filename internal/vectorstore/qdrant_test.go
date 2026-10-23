package vectorstore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"subtitle-rag/internal/domain"
)

func TestPointIDFromChunkID_Parity(t *testing.T) {
	client, err := NewQdrantClient(QdrantOptions{
		NamespaceUUID: "6ba7b811-9dad-11d1-80b4-00c04fd430c8",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	chunkID := "test.vtt__test#1000-61000"
	expectedUUID := "67177303-cffb-5571-bfd5-9cbf72199757"
	actualUUID := client.PointIDFromChunkID(chunkID)

	if actualUUID != expectedUUID {
		t.Errorf("UUIDv5 parity failure: expected %s, got %s", expectedUUID, actualUUID)
	}
}

func TestQdrantClient_EnsureAndUpsertAndSearch(t *testing.T) {
	var createdCollection string
	var upsertedPoints []pointItem

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections":
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"collections": []map[string]any{{"name": "other_col"}},
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/collections/subs_chunks":
			createdCollection = "subs_chunks"
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"result": true})
		case r.Method == http.MethodPut && r.URL.Path == "/collections/subs_chunks/points":
			var req struct {
				Points []pointItem `json:"points"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			upsertedPoints = req.Points
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"result": true})
		case r.Method == http.MethodPost && r.URL.Path == "/collections/subs_chunks/points/search":
			res := qdrantSearchResponse{
				Result: []qdrantSearchHit{
					{
						ID:    "67177303-cffb-5571-bfd5-9cbf72199757",
						Score: 0.95,
						Payload: pointPayload{
							ChunkID:     "test.vtt__test#1000-61000",
							CourseID:    "nodejs",
							FileID:      "test.vtt__test",
							StartMs:     1000,
							EndMs:       3500,
							Text:        "Hello there! General Kenobi.",
							LineIndices: []int{1, 2},
						},
						Vector: []any{0.1, 0.2, 0.3},
					},
				},
			}
			json.NewEncoder(w).Encode(res)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	client, err := NewQdrantClient(QdrantOptions{
		URL:        ts.URL,
		Collection: "subs_chunks",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	// 1. Ensure collection
	if err := client.EnsureCollection(ctx, 768); err != nil {
		t.Fatalf("EnsureCollection failed: %v", err)
	}
	if createdCollection != "subs_chunks" {
		t.Errorf("expected collection subs_chunks created, got %q", createdCollection)
	}

	// 2. Upsert
	chunks := []domain.Chunk{
		{
			ChunkID:     "test.vtt__test#1000-61000",
			CourseID:    "nodejs",
			FileID:      "test.vtt__test",
			StartMs:     1000,
			EndMs:       3500,
			Text:        "Hello there! General Kenobi.",
			LineIndices: []int{1, 2},
			SourceFile:  "test.json",
		},
	}
	vectors := [][]float32{{0.1, 0.2, 0.3}}

	if err := client.Upsert(ctx, chunks, vectors); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}
	if len(upsertedPoints) != 1 {
		t.Fatalf("expected 1 upserted point, got %d", len(upsertedPoints))
	}
	if upsertedPoints[0].ID != "67177303-cffb-5571-bfd5-9cbf72199757" {
		t.Errorf("unexpected point ID: %s", upsertedPoints[0].ID)
	}

	// 3. Search
	course := "nodejs"
	hits, err := client.Search(ctx, []float32{0.1, 0.2, 0.3}, 10, &course, nil, true)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].Score != 0.95 || hits[0].Start != "00:00:01.000" || hits[0].End != "00:00:03.500" {
		t.Errorf("unexpected hit properties: %+v", hits[0])
	}
	if len(hits[0].Vector) != 3 {
		t.Errorf("expected 3 vector elements, got %d", len(hits[0].Vector))
	}
}
