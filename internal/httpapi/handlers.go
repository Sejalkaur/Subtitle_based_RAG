package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"subtitle-rag/internal/config"
	"subtitle-rag/internal/domain"
	"subtitle-rag/internal/retrieval"
)

// APIHandler handles HTTP requests for the Subtitle RAG API.
type APIHandler struct {
	cfg              *config.Config
	retrievalService *retrieval.Service
}

func NewAPIHandler(cfg *config.Config, retrievalService *retrieval.Service) *APIHandler {
	return &APIHandler{
		cfg:              cfg,
		retrievalService: retrievalService,
	}
}

// HealthCheck handles GET /health.
func (h *APIHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"ok":              true,
		"service":         "subtitle-rag-api",
		"version":         "0.1.0",
		"chatModel":       h.cfg.ChatModel,
		"embeddingsModel": h.cfg.EmbeddingsModel,
	}
	writeJSON(w, http.StatusOK, resp)
}

// Root handles GET /.
func (h *APIHandler) Root(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"ok":      true,
		"service": "subtitle-rag-api",
		"message": "Welcome. Try GET /health or POST /api/ask",
	}
	writeJSON(w, http.StatusOK, resp)
}

type errorResponse struct {
	Error   string  `json:"error"`
	Message string  `json:"message"`
	Hint    *string `json:"hint,omitempty"`
}

// Ask handles POST /api/ask.
func (h *APIHandler) Ask(w http.ResponseWriter, r *http.Request) {
	var req domain.AskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"error":   "BadRequest",
			"message": "body must be valid JSON",
		})
		return
	}

	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":      false,
			"error":   "BadRequest",
			"message": "body must have required property 'query'",
		})
		return
	}

	slog.Info("Incoming /api/ask request",
		slog.String("query", req.Query),
		slog.Any("courseId", req.CourseID),
		slog.Any("topK", req.TopK),
		slog.Any("mmrK", req.MMRK),
		slog.Any("minScore", req.MinScore),
	)

	out, err := h.retrievalService.AnswerQuery(r.Context(), req)
	if err != nil {
		slog.Error("/api/ask failed with error",
			slog.String("query", req.Query),
			slog.Any("error", err),
		)

		var hint *string
		errMsg := err.Error()
		if strings.Contains(errMsg, "API_KEY") || strings.Contains(errMsg, "api key") {
			hStr := "Please check your GEMINI_API_KEY in .env"
			hint = &hStr
		} else if strings.Contains(errMsg, "404") || strings.Contains(errMsg, "not found") {
			hStr := "Model not found. Please verify CHAT_MODEL in .env"
			hint = &hStr
		}

		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:   "Failed to process question",
			Message: errMsg,
			Hint:    hint,
		})
		return
	}

	slog.Info("/api/ask completed successfully",
		slog.Int("citationsCount", len(out.Citations)),
		slog.Bool("hasAnswer", out.Answer != ""),
	)

	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
