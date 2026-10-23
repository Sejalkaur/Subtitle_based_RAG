package vectorstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"subtitle-rag/internal/domain"
	"subtitle-rag/internal/subtitle"
)

// QdrantClient implements Store using Qdrant REST API.
type QdrantClient struct {
	url            string
	apiKey         string
	collection     string
	namespaceUUID  uuid.UUID
	httpClient     *http.Client
}

// QdrantOptions holds settings to create QdrantClient.
type QdrantOptions struct {
	URL           string
	APIKey        string
	Collection    string
	NamespaceUUID string
	HTTPClient    *http.Client
}

// NewQdrantClient creates a new Qdrant client.
func NewQdrantClient(opts QdrantOptions) (*QdrantClient, error) {
	url := strings.TrimRight(opts.URL, "/")
	if url == "" {
		url = "http://localhost:6333"
	}
	col := opts.Collection
	if col == "" {
		col = "subs_chunks"
	}

	ns := uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
	if opts.NamespaceUUID != "" {
		if parsed, err := uuid.Parse(opts.NamespaceUUID); err == nil {
			ns = parsed
		}
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &QdrantClient{
		url:           url,
		apiKey:        opts.APIKey,
		collection:    col,
		namespaceUUID: ns,
		httpClient:    httpClient,
	}, nil
}

// PointIDFromChunkID computes deterministic UUIDv5 matching Node.js uuidv5(chunkId, NAMESPACE).
func (q *QdrantClient) PointIDFromChunkID(chunkID string) string {
	return uuid.NewSHA1(q.namespaceUUID, []byte(chunkID)).String()
}

func (q *QdrantClient) doRequest(ctx context.Context, method, path string, body any, out any) (int, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	reqURL := fmt.Sprintf("%s%s", q.url, path)
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if q.apiKey != "" {
		req.Header.Set("api-key", q.apiKey)
	}

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("qdrant request error (%s %s): %w", method, path, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return resp.StatusCode, fmt.Errorf("qdrant api error (status %d): %s", resp.StatusCode, string(respBytes))
	}

	if out != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return resp.StatusCode, fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return resp.StatusCode, nil
}

type collectionListResponse struct {
	Result struct {
		Collections []struct {
			Name string `json:"name"`
		} `json:"collections"`
	} `json:"result"`
}

// GetCollections returns names of all collections.
func (q *QdrantClient) GetCollections(ctx context.Context) ([]string, error) {
	var res collectionListResponse
	_, err := q.doRequest(ctx, http.MethodGet, "/collections", nil, &res)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(res.Result.Collections))
	for i, c := range res.Result.Collections {
		names[i] = c.Name
	}
	return names, nil
}

// EnsureCollection verifies the collection exists with right dimension and Cosine metric; creates it if missing.
func (q *QdrantClient) EnsureCollection(ctx context.Context, dim int) error {
	names, err := q.GetCollections(ctx)
	if err != nil {
		return err
	}

	for _, n := range names {
		if n == q.collection {
			return nil
		}
	}

	slog.Info("[Qdrant] Creating collection...",
		slog.String("collection", q.collection),
		slog.Int("dimension", dim),
	)

	createBody := map[string]any{
		"vectors": map[string]any{
			"size":     dim,
			"distance": "Cosine",
		},
		"optimizers_config": map[string]any{
			"default_segment_number": 2,
		},
	}

	_, err = q.doRequest(ctx, http.MethodPut, "/collections/"+q.collection, createBody, nil)
	return err
}

type pointPayload struct {
	ChunkID     string `json:"chunkId"`
	CourseID    string `json:"courseId"`
	FileID      string `json:"fileId"`
	StartMs     int64  `json:"startMs"`
	EndMs       int64  `json:"endMs"`
	Text        string `json:"text"`
	LineIndices []int  `json:"lineIndices"`
	SourceFile  string `json:"sourceFile"`
}

type pointItem struct {
	ID      string       `json:"id"`
	Vector  []float32    `json:"vector"`
	Payload pointPayload `json:"payload"`
}

// Upsert adds or updates points in the collection idempotently using UUIDv5 IDs.
func (q *QdrantClient) Upsert(ctx context.Context, chunks []domain.Chunk, vectors [][]float32) error {
	if len(chunks) == 0 {
		return nil
	}
	if len(chunks) != len(vectors) {
		return fmt.Errorf("chunks count (%d) and vectors count (%d) do not match", len(chunks), len(vectors))
	}

	points := make([]pointItem, len(chunks))
	for i, c := range chunks {
		points[i] = pointItem{
			ID:     q.PointIDFromChunkID(c.ChunkID),
			Vector: vectors[i],
			Payload: pointPayload{
				ChunkID:     c.ChunkID,
				CourseID:    c.CourseID,
				FileID:      c.FileID,
				StartMs:     c.StartMs,
				EndMs:       c.EndMs,
				Text:        c.Text,
				LineIndices: c.LineIndices,
				SourceFile:  c.SourceFile,
			},
		}
	}

	body := map[string]any{
		"points": points,
	}

	_, err := q.doRequest(ctx, http.MethodPut, fmt.Sprintf("/collections/%s/points", q.collection), body, nil)
	return err
}

type qdrantSearchHit struct {
	ID      any          `json:"id"`
	Score   float64      `json:"score"`
	Payload pointPayload `json:"payload"`
	Vector  any          `json:"vector,omitempty"`
}

type qdrantSearchResponse struct {
	Result []qdrantSearchHit `json:"result"`
}

func extractVectorFloats(raw any) []float32 {
	if raw == nil {
		return nil
	}
	// Case 1: []any containing float64
	if arr, ok := raw.([]any); ok {
		res := make([]float32, len(arr))
		for i, v := range arr {
			if f, ok := v.(float64); ok {
				res[i] = float32(f)
			}
		}
		return res
	}
	// Case 2: map of named vectors
	if m, ok := raw.(map[string]any); ok {
		for _, v := range m {
			return extractVectorFloats(v)
		}
	}
	return nil
}

// Search performs semantic similarity search against the collection.
func (q *QdrantClient) Search(ctx context.Context, vector []float32, limit int, courseID, fileID *string, withVector bool) ([]domain.SearchHit, error) {
	reqBody := map[string]any{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
		"with_vector":  withVector,
	}

	var mustClauses []map[string]any
	if courseID != nil && *courseID != "" {
		mustClauses = append(mustClauses, map[string]any{
			"key": "courseId",
			"match": map[string]any{
				"value": *courseID,
			},
		})
	}
	if fileID != nil && *fileID != "" {
		mustClauses = append(mustClauses, map[string]any{
			"key": "fileId",
			"match": map[string]any{
				"value": *fileID,
			},
		})
	}
	if len(mustClauses) > 0 {
		reqBody["filter"] = map[string]any{
			"must": mustClauses,
		}
	}

	var resp qdrantSearchResponse
	_, err := q.doRequest(ctx, http.MethodPost, fmt.Sprintf("/collections/%s/points/search", q.collection), reqBody, &resp)
	if err != nil {
		return nil, err
	}

	hits := make([]domain.SearchHit, len(resp.Result))
	for i, r := range resp.Result {
		preview := r.Payload.Text
		if len(preview) > 180 {
			preview = preview[:180] + "…"
		}

		hits[i] = domain.SearchHit{
			FileID:      r.Payload.FileID,
			CourseID:    r.Payload.CourseID,
			StartMs:     r.Payload.StartMs,
			EndMs:       r.Payload.EndMs,
			Start:       subtitle.MsToTimestamp(r.Payload.StartMs),
			End:         subtitle.MsToTimestamp(r.Payload.EndMs),
			Preview:     preview,
			Text:        r.Payload.Text,
			LineIndices: r.Payload.LineIndices,
			Score:       r.Score,
			Vector:      extractVectorFloats(r.Vector),
		}
	}

	return hits, nil
}
