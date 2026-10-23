package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"subtitle-rag/internal/config"
	"subtitle-rag/internal/domain"
	"subtitle-rag/internal/embedding"
	"subtitle-rag/internal/observability"
	"subtitle-rag/internal/retrieval"
	"subtitle-rag/internal/vectorstore"
)

func main() {
	queryFlag := flag.String("query", "", "Search query text")
	courseFlag := flag.String("course", "", "Filter by courseId")
	fileFlag := flag.String("file", "", "Filter by fileId")
	topKFlag := flag.Int("top", 80, "Top K candidates to fetch from Qdrant")
	keepFlag := flag.Int("keep", 12, "Number of candidates to keep after MMR")
	minScoreFlag := flag.Float64("min", 0.52, "Confidence threshold")
	noMMRFlag := flag.Bool("no-mmr", false, "Disable MMR diversification")
	noStrictFlag := flag.Bool("no-strict", false, "Disable strict MMR")
	printLimitFlag := flag.Int("print", 10, "Maximum results to display")
	flag.Parse()

	query := *queryFlag
	if query == "" && flag.NArg() > 0 {
		query = flag.Arg(0)
	}

	if query == "" {
		fmt.Println("Usage: go run cmd/search/main.go \"<query>\" [--course <id>] [--file <id>] [--top 80] [--keep 12] [--min 0.52] [--no-mmr] [--no-strict]")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	_ = observability.InitLogger("warn", cfg.NodeEnv)

	embedder, err := embedding.NewGeminiClient(embedding.GeminiOptions{
		APIKey: cfg.GeminiAPIKey,
		Model:  cfg.EmbeddingsModel,
		Dim:    cfg.EmbeddingsDim,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Embedder error: %v\n", err)
		os.Exit(1)
	}

	store, err := vectorstore.NewQdrantClient(vectorstore.QdrantOptions{
		URL:           cfg.QdrantURL,
		APIKey:        cfg.QdrantAPIKey,
		Collection:    cfg.QdrantCollection,
		NamespaceUUID: cfg.ChunkIDNamespace,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Qdrant error: %v\n", err)
		os.Exit(1)
	}

	svc := retrieval.NewService(embedder, store, nil)

	var coursePtr *string
	if *courseFlag != "" {
		coursePtr = courseFlag
	}
	var filePtr *string
	if *fileFlag != "" {
		filePtr = fileFlag
	}

	res, err := svc.SearchChunks(context.Background(), domain.SearchOptions{
		Query:     query,
		CourseID:  coursePtr,
		FileID:    filePtr,
		TopK:      *topKFlag,
		MMRK:      *keepFlag,
		UseMMR:    !*noMMRFlag,
		StrictMMR: !*noStrictFlag,
		MinScore:  *minScoreFlag,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nQuery:", res.Query)
	fmt.Printf("Meta : %+v\n", res.Meta)

	if len(res.Results) == 0 {
		fmt.Println("\nI don’t know, but I could help in other areas.")
		return
	}

	limit := *printLimitFlag
	if limit > len(res.Results) {
		limit = len(res.Results)
	}

	fmt.Printf("\nTop results (showing %d of %d):\n", limit, len(res.Results))
	for i := 0; i < limit; i++ {
		r := res.Results[i]
		fmt.Printf("\n#%d  [%s] %s  %s–%s\n", i+1, r.CourseID, r.FileID, r.Start, r.End)
		fmt.Printf("     score: %.4f\n", r.Score)
		fmt.Printf("     text : %s\n", r.Preview)
	}
}
