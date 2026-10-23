package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var retrySecRegex = regexp.MustCompile(`(?i)retry in ([0-9.]+)s`)

// GeminiClient implements Client using Google Generative Language API.
type GeminiClient struct {
	apiKey     string
	model      string
	dim        int
	baseURL    string
	httpClient *http.Client
}

// GeminiOptions holds options to initialize GeminiClient.
type GeminiOptions struct {
	APIKey     string
	Model      string
	Dim        int
	BaseURL    string
	HTTPClient *http.Client
}

// NewGeminiClient creates a new Gemini embedding client.
func NewGeminiClient(opts GeminiOptions) (*GeminiClient, error) {
	if opts.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is missing")
	}
	model := opts.Model
	if model == "" {
		model = "gemini-embedding-2"
	}
	dim := opts.Dim
	if dim <= 0 {
		dim = 768
	}
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	return &GeminiClient{
		apiKey:     opts.APIKey,
		model:      model,
		dim:        dim,
		baseURL:    baseURL,
		httpClient: httpClient,
	}, nil
}

func (c *GeminiClient) Dimension() int {
	return c.dim
}

func (c *GeminiClient) Model() string {
	return c.model
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type embedRequest struct {
	Content              geminiContent `json:"content"`
	TaskType             string        `json:"taskType"`
	OutputDimensionality int           `json:"outputDimensionality"`
}

type embedResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func (c *GeminiClient) embedWithRetry(ctx context.Context, text, taskType string, maxRetries int) ([]float32, error) {
	attempt := 0
	url := fmt.Sprintf("%s/models/%s:embedContent?key=%s", c.baseURL, c.model, c.apiKey)

	reqBody := embedRequest{
		Content: geminiContent{
			Parts: []geminiPart{{Text: text}},
		},
		TaskType:             taskType,
		OutputDimensionality: c.dim,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	for {
		attempt++
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			if attempt <= maxRetries && ctx.Err() == nil {
				slog.Warn("HTTP request failed, retrying...", slog.Int("attempt", attempt), slog.Any("error", err))
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(2 * time.Second):
					continue
				}
			}
			return nil, err
		}

		respBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		var parsed embedResponse
		if err := json.Unmarshal(respBytes, &parsed); err != nil {
			return nil, fmt.Errorf("unmarshal response (status %d): %w", resp.StatusCode, err)
		}

		isRateLimit := resp.StatusCode == http.StatusTooManyRequests ||
			(parsed.Error != nil && (parsed.Error.Code == 429 ||
				strings.Contains(parsed.Error.Message, "429") ||
				strings.Contains(parsed.Error.Message, "Quota exceeded") ||
				strings.Contains(parsed.Error.Message, "Too Many Requests")))

		if isRateLimit && attempt <= maxRetries {
			delay := 31 * time.Second
			if parsed.Error != nil {
				match := retrySecRegex.FindStringSubmatch(parsed.Error.Message)
				if len(match) > 1 {
					if sec, err := strconv.ParseFloat(match[1], 64); err == nil {
						delay = time.Duration(sec*1000)*time.Millisecond + 2*time.Second
					}
				}
			}
			slog.Warn("[Embeddings] Rate limit hit. Pausing before retry...",
				slog.Float64("delaySeconds", delay.Seconds()),
				slog.Int("attempt", attempt),
				slog.Int("maxRetries", maxRetries),
			)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				continue
			}
		}

		if resp.StatusCode != http.StatusOK {
			errMsg := string(respBytes)
			if parsed.Error != nil && parsed.Error.Message != "" {
				errMsg = parsed.Error.Message
			}
			return nil, fmt.Errorf("gemini api error (status %d): %s", resp.StatusCode, errMsg)
		}

		return parsed.Embedding.Values, nil
	}
}

// EmbedOne generates embedding for a single document chunk.
func (c *GeminiClient) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	return c.embedWithRetry(ctx, text, "RETRIEVAL_DOCUMENT", 5)
}

// EmbedMany generates embeddings for multiple chunks in sub-batches of 4 with 200ms delay.
func (c *GeminiClient) EmbedMany(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	start := time.Now()
	slog.Info("[Embeddings] Generating embeddings for chunk batch...",
		slog.Int("count", len(texts)),
		slog.String("model", c.model),
	)

	const concurrency = 4
	out := make([][]float32, len(texts))

	for i := 0; i < len(texts); i += concurrency {
		end := i + concurrency
		if end > len(texts) {
			end = len(texts)
		}

		var wg sync.WaitGroup
		errChan := make(chan error, end-i)

		for j := i; j < end; j++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				vec, err := c.embedWithRetry(ctx, texts[idx], "RETRIEVAL_DOCUMENT", 5)
				if err != nil {
					errChan <- fmt.Errorf("chunk %d: %w", idx, err)
					return
				}
				out[idx] = vec
			}(j)
		}

		wg.Wait()
		close(errChan)

		if err := <-errChan; err != nil {
			return nil, err
		}

		if end < len(texts) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}
		}
	}

	slog.Info("[Embeddings] Batch embeddings complete",
		slog.Int("count", len(texts)),
		slog.Duration("duration", time.Since(start)),
	)
	return out, nil
}

// EmbedQuery generates embedding for a search query.
func (c *GeminiClient) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	start := time.Now()
	vec, err := c.embedWithRetry(ctx, text, "RETRIEVAL_QUERY", 5)
	if err != nil {
		slog.Error("[Embeddings] Query embedding error",
			slog.Any("error", err),
			slog.String("model", c.model),
		)
		return nil, err
	}
	slog.Info("[Embeddings] Query vector generated",
		slog.Duration("duration", time.Since(start)),
		slog.String("model", c.model),
	)
	return vec, nil
}
