package retrieval

import (
	"context"
	"fmt"
	"log/slog"

	"subtitle-rag/internal/domain"
	"subtitle-rag/internal/embedding"
	"subtitle-rag/internal/vectorstore"
)

// Service provides semantic search and RAG question answering.
type Service struct {
	embedder embedding.Client
	store    vectorstore.Store
	llm      LLMClient
}

func NewService(embedder embedding.Client, store vectorstore.Store, llm LLMClient) *Service {
	return &Service{
		embedder: embedder,
		store:    store,
		llm:      llm,
	}
}

// SearchChunks searches subtitle chunks by semantic similarity, with confidence gating and MMR diversification.
func (s *Service) SearchChunks(ctx context.Context, opts domain.SearchOptions) (domain.SearchResult, error) {
	if opts.Query == "" {
		return domain.SearchResult{}, fmt.Errorf("query is required")
	}
	if opts.TopK <= 0 {
		opts.TopK = 80
	}
	if opts.MMRK <= 0 {
		opts.MMRK = 12
	}
	if opts.MinScore <= 0 {
		opts.MinScore = 0.52
	}

	filterMap := map[string]*string{
		"courseId": opts.CourseID,
		"fileId":   opts.FileID,
	}

	slog.Info("[Retrieval] Starting chunk search...",
		slog.String("query", opts.Query),
		slog.Any("courseId", opts.CourseID),
		slog.Int("topK", opts.TopK),
		slog.Int("mmrK", opts.MMRK),
	)

	qVec, err := s.embedder.EmbedQuery(ctx, opts.Query)
	if err != nil {
		return domain.SearchResult{}, fmt.Errorf("embed query: %w", err)
	}

	hits, err := s.store.Search(ctx, qVec, opts.TopK, opts.CourseID, opts.FileID, opts.UseMMR)
	if err != nil {
		slog.Error("[Retrieval] Vector search failed", slog.Any("error", err))
		return domain.SearchResult{}, fmt.Errorf("store search: %w", err)
	}

	slog.Info("[Retrieval] Candidates returned", slog.Int("count", len(hits)))

	if len(hits) == 0 {
		slog.Info("[Retrieval] No candidates matched query/filter")
		return domain.SearchResult{
			Query:   opts.Query,
			Results: []domain.SearchHit{},
			Meta: domain.SearchMeta{
				TopK:   opts.TopK,
				MMR:    opts.UseMMR,
				Reason: "no_candidates",
				Filter: filterMap,
			},
		}, nil
	}

	bestScore := hits[0].Score
	if bestScore < opts.MinScore {
		slog.Info("[Retrieval] Score below confidence threshold",
			slog.Float64("bestScore", bestScore),
			slog.Float64("minScore", opts.MinScore),
		)
		return domain.SearchResult{
			Query:   opts.Query,
			Results: []domain.SearchHit{},
			Meta: domain.SearchMeta{
				TopK:      opts.TopK,
				MMR:       opts.UseMMR,
				Reason:    "low_confidence",
				BestScore: &bestScore,
				MinScore:  &opts.MinScore,
				Filter:    filterMap,
			},
		}, nil
	}

	finalHits := hits
	if opts.UseMMR {
		picked := MMR(hits, qVec, minInt(opts.MMRK, len(hits)), 0.6)
		slog.Info("[Retrieval] MMR diversification complete",
			slog.Int("pickedCount", len(picked)),
			slog.Int("requestedMMRK", opts.MMRK),
		)

		if len(picked) == 0 && opts.StrictMMR {
			slog.Warn("[Retrieval] MMR returned 0 results with strictMMR=true")
			return domain.SearchResult{
				Query:   opts.Query,
				Results: []domain.SearchHit{},
				Meta: domain.SearchMeta{
					TopK:      opts.TopK,
					MMR:       true,
					Reason:    "mmr_empty",
					BestScore: &bestScore,
					MinScore:  &opts.MinScore,
					Filter:    filterMap,
				},
			}, nil
		}

		if len(picked) > 0 {
			finalHits = picked
		}
	}

	kept := len(finalHits)
	return domain.SearchResult{
		Query:   opts.Query,
		Results: finalHits,
		Meta: domain.SearchMeta{
			TopK:      opts.TopK,
			MMR:       opts.UseMMR,
			MMRKept:   &kept,
			BestScore: &bestScore,
			MinScore:  &opts.MinScore,
			Filter:    filterMap,
		},
	}, nil
}

// AnswerQuery answers a question grounded on subtitles, matching Node.js answerQuery().
func (s *Service) AnswerQuery(ctx context.Context, req domain.AskRequest) (domain.AskResponse, error) {
	topK := 80
	if req.TopK != nil && *req.TopK > 0 {
		topK = *req.TopK
	}
	mmrK := 12
	if req.MMRK != nil && *req.MMRK > 0 {
		mmrK = *req.MMRK
	}
	minScore := 0.55
	if req.MinScore != nil {
		minScore = *req.MinScore
	}
	strictMMR := true
	if req.StrictMMR != nil {
		strictMMR = *req.StrictMMR
	}

	ret, err := s.SearchChunks(ctx, domain.SearchOptions{
		Query:     req.Query,
		CourseID:  req.CourseID,
		TopK:      topK,
		MMRK:      mmrK,
		UseMMR:    true,
		StrictMMR: strictMMR,
		MinScore:  minScore,
	})
	if err != nil {
		return domain.AskResponse{}, err
	}

	used := domain.AskUsed{
		Query:     req.Query,
		CourseID:  req.CourseID,
		TopK:      topK,
		MMRK:      mmrK,
		MinScore:  minScore,
		StrictMMR: strictMMR,
		Meta:      ret.Meta,
	}

	if len(ret.Results) == 0 {
		return domain.AskResponse{
			Answer:    "I don’t know, but I could help in other areas.",
			Citations: []domain.Citation{},
			Used:      used,
		}, nil
	}

	prompt := BuildPrompt(req.Query, ret.Results)

	answer := ""
	if s.llm != nil {
		respText, err := s.llm.Generate(ctx, prompt.System, prompt.User, 0.2)
		if err != nil {
			return domain.AskResponse{}, fmt.Errorf("llm generation: %w", err)
		}
		answer = respText
	}

	citations := make([]domain.Citation, len(ret.Results))
	for i, r := range ret.Results {
		citations[i] = domain.Citation{
			CourseID: r.CourseID,
			FileID:   r.FileID,
			Start:    r.Start,
			End:      r.End,
			StartMs:  r.StartMs,
			EndMs:    r.EndMs,
			Score:    r.Score,
		}
	}

	return domain.AskResponse{
		Answer:    answer,
		Citations: citations,
		Used:      used,
	}, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
