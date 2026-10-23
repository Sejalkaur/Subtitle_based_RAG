package retrieval

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
)

// LLMClient defines the chat generation contract.
type LLMClient interface {
	Generate(ctx context.Context, system, user string, temperature float64) (string, error)
	Model() string
}

// GeminiLLMClient implements LLMClient using Google Generative Language API.
type GeminiLLMClient struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

type GeminiLLMOptions struct {
	APIKey     string
	Model      string
	BaseURL    string
	HTTPClient *http.Client
}

func NewGeminiLLMClient(opts GeminiLLMOptions) (*GeminiLLMClient, error) {
	if opts.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is missing")
	}
	model := opts.Model
	if model == "" {
		model = "gemini-3.8-flash"
	}
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	return &GeminiLLMClient{
		apiKey:     opts.APIKey,
		model:      model,
		baseURL:    baseURL,
		httpClient: httpClient,
	}, nil
}

func (c *GeminiLLMClient) Model() string {
	return c.model
}

type geminiChatPart struct {
	Text string `json:"text"`
}

type geminiChatContent struct {
	Role  string           `json:"role"`
	Parts []geminiChatPart `json:"parts"`
}

type geminiGenerateRequest struct {
	Contents         []geminiChatContent `json:"contents"`
	GenerationConfig map[string]any      `json:"generationConfig,omitempty"`
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// Generate sends prompt to Gemini model with fallback on 503, matching Node.js chat().
func (c *GeminiLLMClient) Generate(ctx context.Context, system, user string, temperature float64) (string, error) {
	// Fold system message into contents
	contents := []geminiChatContent{
		{
			Role:  "user",
			Parts: []geminiChatPart{{Text: system}},
		},
		{
			Role:  "user",
			Parts: []geminiChatPart{{Text: user}},
		},
	}

	reqBody := geminiGenerateRequest{
		Contents: contents,
		GenerationConfig: map[string]any{
			"temperature": temperature,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal generate request: %w", err)
	}

	modelsToTry := []string{c.model}
	if c.model != "gemini-3.5-flash" {
		modelsToTry = append(modelsToTry, "gemini-3.5-flash")
	}

	var lastErr error
	start := time.Now()

	for _, mName := range modelsToTry {
		slog.Info("[LLM] Sending prompt to Gemini model...",
			slog.String("model", mName),
			slog.Int("messageCount", len(contents)),
		)

		url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, mName, c.apiKey)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}

		respBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response body: %w", err)
			continue
		}

		var parsed geminiGenerateResponse
		if err := json.Unmarshal(respBytes, &parsed); err != nil {
			lastErr = fmt.Errorf("unmarshal response (status %d): %w", resp.StatusCode, err)
			continue
		}

		is503 := resp.StatusCode == http.StatusServiceUnavailable ||
			(parsed.Error != nil && (parsed.Error.Code == 503 ||
				strings.Contains(parsed.Error.Message, "503") ||
				strings.Contains(parsed.Error.Message, "high demand")))

		if is503 {
			slog.Warn(fmt.Sprintf(`[LLM] Model "%s" busy (503 High Demand). Retrying with fallback model...`, mName))
			lastErr = fmt.Errorf("model busy (503)")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			errMsg := string(respBytes)
			if parsed.Error != nil && parsed.Error.Message != "" {
				errMsg = parsed.Error.Message
			}
			lastErr = fmt.Errorf("gemini generate error (status %d): %s", resp.StatusCode, errMsg)
			break
		}

		if len(parsed.Candidates) > 0 && len(parsed.Candidates[0].Content.Parts) > 0 {
			answer := parsed.Candidates[0].Content.Parts[0].Text
			slog.Info("[LLM] Response received successfully",
				slog.String("model", mName),
				slog.Duration("duration", time.Since(start)),
				slog.Int("responseLength", len(answer)),
			)
			return answer, nil
		}

		return "", nil
	}

	return "", lastErr
}
